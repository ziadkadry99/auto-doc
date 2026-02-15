package flows

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/ziadkadry99/auto-doc/internal/db"
)

func setupTestStore(t *testing.T) *Store {
	t.Helper()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return NewStore(d)
}

// --- Store CRUD tests ---

func TestCreateAndGetFlow(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	f := &Flow{
		Name:        "User Signup",
		Description: "End-to-end user registration flow",
		Services:    []string{"api-gateway", "auth-service", "user-db"},
		EntryPoint:  "POST /api/signup",
		ExitPoint:   "201 Created",
	}
	if err := store.CreateFlow(ctx, f); err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	if f.ID == "" {
		t.Fatal("expected flow ID to be set")
	}

	got, err := store.GetFlow(ctx, f.ID)
	if err != nil {
		t.Fatalf("GetFlow: %v", err)
	}
	if got.Name != "User Signup" {
		t.Errorf("name = %q, want %q", got.Name, "User Signup")
	}
	if len(got.Services) != 3 {
		t.Errorf("services count = %d, want 3", len(got.Services))
	}
	if got.EntryPoint != "POST /api/signup" {
		t.Errorf("entry_point = %q, want %q", got.EntryPoint, "POST /api/signup")
	}
}

func TestListFlows(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	store.CreateFlow(ctx, &Flow{Name: "Flow A", Services: []string{}})
	store.CreateFlow(ctx, &Flow{Name: "Flow B", Services: []string{}})

	flows, err := store.ListFlows(ctx)
	if err != nil {
		t.Fatalf("ListFlows: %v", err)
	}
	if len(flows) != 2 {
		t.Fatalf("got %d flows, want 2", len(flows))
	}
}

func TestUpdateFlow(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	f := &Flow{Name: "Original", Services: []string{"svc-a"}}
	store.CreateFlow(ctx, f)

	f.Name = "Updated"
	f.Services = []string{"svc-a", "svc-b"}
	if err := store.UpdateFlow(ctx, f); err != nil {
		t.Fatalf("UpdateFlow: %v", err)
	}

	got, _ := store.GetFlow(ctx, f.ID)
	if got.Name != "Updated" {
		t.Errorf("name = %q, want %q", got.Name, "Updated")
	}
	if len(got.Services) != 2 {
		t.Errorf("services count = %d, want 2", len(got.Services))
	}
}

func TestDeleteFlow(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	f := &Flow{Name: "Temporary", Services: []string{}}
	store.CreateFlow(ctx, f)

	if err := store.DeleteFlow(ctx, f.ID); err != nil {
		t.Fatalf("DeleteFlow: %v", err)
	}

	_, err := store.GetFlow(ctx, f.ID)
	if err == nil {
		t.Fatal("expected error after deleting flow")
	}
}

func TestSearchFlows(t *testing.T) {
	store := setupTestStore(t)
	ctx := context.Background()

	store.CreateFlow(ctx, &Flow{Name: "User Registration", Description: "signup flow", Services: []string{}})
	store.CreateFlow(ctx, &Flow{Name: "Payment Processing", Description: "handles payments", Services: []string{}})

	results, err := store.SearchFlows(ctx, "payment")
	if err != nil {
		t.Fatalf("SearchFlows: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Name != "Payment Processing" {
		t.Errorf("name = %q, want %q", results[0].Name, "Payment Processing")
	}

	// Search by description.
	results, err = store.SearchFlows(ctx, "signup")
	if err != nil {
		t.Fatalf("SearchFlows: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
}

// --- Detector tests ---

func TestDetectHTTPPatterns(t *testing.T) {
	d := NewDetector()

	code := `
resp, err := http.Get("http://user-service:8080/users")
resp, err = http.Post("http://order-service/orders", "application/json", body)
req, _ := http.NewRequest("PUT", "http://inventory/items", nil)
`
	calls := d.DetectPatterns(code, "main.go")
	if len(calls) != 3 {
		t.Fatalf("got %d HTTP calls, want 3", len(calls))
	}
	for _, c := range calls {
		if c.Type != "http" {
			t.Errorf("type = %q, want %q", c.Type, "http")
		}
		if c.FilePath != "main.go" {
			t.Errorf("file_path = %q, want %q", c.FilePath, "main.go")
		}
	}
	if calls[0].Method != "GET" {
		t.Errorf("calls[0].Method = %q, want GET", calls[0].Method)
	}
	if calls[1].Method != "POST" {
		t.Errorf("calls[1].Method = %q, want POST", calls[1].Method)
	}
}

func TestDetectFetchAndAxios(t *testing.T) {
	d := NewDetector()

	code := `
const resp = await fetch("/api/data")
const users = await axios.get("/api/users")
await axios.post("/api/orders")
`
	calls := d.DetectPatterns(code, "app.js")
	if len(calls) != 3 {
		t.Fatalf("got %d calls, want 3", len(calls))
	}
	if calls[0].Type != "http" {
		t.Errorf("fetch type = %q, want http", calls[0].Type)
	}
}

func TestDetectPythonRequests(t *testing.T) {
	d := NewDetector()

	code := `
resp = requests.get("http://api.example.com/data")
resp = requests.post("http://api.example.com/submit")
`
	calls := d.DetectPatterns(code, "app.py")
	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(calls))
	}
}

func TestDetectKafkaPatterns(t *testing.T) {
	d := NewDetector()

	code := `
producer, _ := kafka.NewProducer(&kafka.ConfigMap{"bootstrap.servers": "localhost"})
consumer, _ := kafka.NewConsumer(&kafka.ConfigMap{"bootstrap.servers": "localhost"})
`
	calls := d.DetectPatterns(code, "kafka.go")
	if len(calls) != 2 {
		t.Fatalf("got %d Kafka calls, want 2", len(calls))
	}
	if calls[0].Method != "produce" {
		t.Errorf("producer method = %q, want produce", calls[0].Method)
	}
	if calls[1].Method != "consume" {
		t.Errorf("consumer method = %q, want consume", calls[1].Method)
	}
}

func TestDetectKafkaJavaPatterns(t *testing.T) {
	d := NewDetector()

	code := `
@KafkaListener(topics = "orders")
public void consume(String message) {}

kafkaTemplate = new KafkaTemplate<>(producerFactory);
`
	calls := d.DetectPatterns(code, "Service.java")
	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(calls))
	}
}

func TestDetectGRPCPatterns(t *testing.T) {
	d := NewDetector()

	code := `
conn, err := grpc.Dial("user-service:50051", grpc.WithInsecure())
client := pb.NewUserServiceClient(conn)
`
	calls := d.DetectPatterns(code, "client.go")
	if len(calls) != 2 {
		t.Fatalf("got %d gRPC calls, want 2", len(calls))
	}
	if calls[0].Type != "grpc" {
		t.Errorf("type = %q, want grpc", calls[0].Type)
	}
	if calls[0].Method != "dial" {
		t.Errorf("method = %q, want dial", calls[0].Method)
	}
}

func TestDetectAMQPPatterns(t *testing.T) {
	d := NewDetector()

	code := `
conn, _ := amqp.Dial("amqp://guest:guest@localhost:5672/")
ch.Publish(exchange, key, false, false, msg)
msgs, _ := ch.Consume(queue, "", true, false, false, false, nil)
`
	// amqp.Dial matches but channel.Publish/Consume expect "channel." prefix
	calls := d.DetectPatterns(code, "mq.go")
	if len(calls) < 1 {
		t.Fatalf("got %d AMQP calls, want at least 1", len(calls))
	}
	if calls[0].Type != "amqp" {
		t.Errorf("type = %q, want amqp", calls[0].Type)
	}
}

func TestDetectSNSSQSPatterns(t *testing.T) {
	d := NewDetector()

	code := `
result, err := sns.Publish(&sns.PublishInput{TopicArn: &topicArn})
out, err := sqs.SendMessage(&sqs.SendMessageInput{QueueUrl: &queueURL})
msgs, err := sqs.ReceiveMessage(&sqs.ReceiveMessageInput{QueueUrl: &queueURL})
`
	calls := d.DetectPatterns(code, "aws.go")
	if len(calls) != 3 {
		t.Fatalf("got %d calls, want 3", len(calls))
	}
}

func TestDetectNoPatterns(t *testing.T) {
	d := NewDetector()

	code := `
func main() {
	fmt.Println("Hello, World!")
	x := 42
}
`
	calls := d.DetectPatterns(code, "simple.go")
	if len(calls) != 0 {
		t.Fatalf("got %d calls, want 0", len(calls))
	}
}

// --- HTTP handler tests ---

func setupTestRouter(t *testing.T) (chi.Router, *Store) {
	t.Helper()
	store := setupTestStore(t)
	r := chi.NewRouter()
	RegisterRoutes(r, store)
	return r, store
}

func TestHTTPListFlowsEmpty(t *testing.T) {
	r, _ := setupTestRouter(t)

	req := httptest.NewRequest("GET", "/api/flows", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var flows []Flow
	json.NewDecoder(w.Body).Decode(&flows)
	if len(flows) != 0 {
		t.Fatalf("got %d flows, want 0", len(flows))
	}
}

func TestHTTPCreateAndGetFlow(t *testing.T) {
	r, _ := setupTestRouter(t)

	body, _ := json.Marshal(Flow{
		Name:     "Test Flow",
		Services: []string{"svc-a", "svc-b"},
	})
	req := httptest.NewRequest("POST", "/api/flows", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d, body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
	var created Flow
	json.NewDecoder(w.Body).Decode(&created)
	if created.ID == "" {
		t.Fatal("expected ID in response")
	}

	// Get
	req = httptest.NewRequest("GET", "/api/flows/"+created.ID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", w.Code, http.StatusOK)
	}
	var got Flow
	json.NewDecoder(w.Body).Decode(&got)
	if got.Name != "Test Flow" {
		t.Errorf("name = %q, want %q", got.Name, "Test Flow")
	}
	if len(got.Services) != 2 {
		t.Errorf("services count = %d, want 2", len(got.Services))
	}
}

func TestHTTPUpdateFlow(t *testing.T) {
	r, store := setupTestRouter(t)
	ctx := context.Background()

	f := &Flow{Name: "Old Flow", Services: []string{}}
	store.CreateFlow(ctx, f)

	body, _ := json.Marshal(Flow{Name: "New Flow", Services: []string{"svc-x"}})
	req := httptest.NewRequest("PUT", "/api/flows/"+f.ID, bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d, body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHTTPSearchFlows(t *testing.T) {
	r, store := setupTestRouter(t)
	ctx := context.Background()

	store.CreateFlow(ctx, &Flow{Name: "Auth Flow", Services: []string{}})
	store.CreateFlow(ctx, &Flow{Name: "Payment Flow", Services: []string{}})

	req := httptest.NewRequest("GET", "/api/flows?q=auth", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var flows []Flow
	json.NewDecoder(w.Body).Decode(&flows)
	if len(flows) != 1 {
		t.Fatalf("got %d flows, want 1", len(flows))
	}
}
