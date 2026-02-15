package flows

import (
	"regexp"
	"strings"
)

// Detector detects cross-service communication patterns in source code.
type Detector struct{}

// NewDetector creates a new pattern detector.
func NewDetector() *Detector {
	return &Detector{}
}

type patternDef struct {
	typ     string
	re      *regexp.Regexp
	method  string
	targets func([]string) string
}

var httpPatterns = []*patternDef{
	{typ: "http", re: regexp.MustCompile(`http\.Get\(([^)]+)\)`), method: "GET", targets: group1},
	{typ: "http", re: regexp.MustCompile(`http\.Post\(([^,]+)`), method: "POST", targets: group1},
	{typ: "http", re: regexp.MustCompile(`http\.NewRequest\(\s*"([A-Z]+)"\s*,\s*([^,]+)`), method: "dynamic", targets: group2},
	{typ: "http", re: regexp.MustCompile(`fetch\(([^)]+)\)`), method: "GET", targets: group1},
	{typ: "http", re: regexp.MustCompile(`axios\.(get|post|put|delete)\(([^)]+)\)`), method: "dynamic", targets: group2},
	{typ: "http", re: regexp.MustCompile(`requests\.(get|post|put|delete)\(([^)]+)\)`), method: "dynamic", targets: group2},
}

var kafkaPatterns = []*patternDef{
	{typ: "kafka", re: regexp.MustCompile(`kafka\.NewProducer\(([^)]*)\)`), method: "produce", targets: group1},
	{typ: "kafka", re: regexp.MustCompile(`kafka\.NewConsumer\(([^)]*)\)`), method: "consume", targets: group1},
	{typ: "kafka", re: regexp.MustCompile(`KafkaTemplate`), method: "produce", targets: literal("KafkaTemplate")},
	{typ: "kafka", re: regexp.MustCompile(`@KafkaListener`), method: "consume", targets: literal("KafkaListener")},
	{typ: "kafka", re: regexp.MustCompile(`sarama\.NewSyncProducer`), method: "produce", targets: literal("sarama")},
	{typ: "kafka", re: regexp.MustCompile(`sarama\.NewConsumer`), method: "consume", targets: literal("sarama")},
}

var grpcPatterns = []*patternDef{
	{typ: "grpc", re: regexp.MustCompile(`grpc\.Dial\(([^)]+)\)`), method: "dial", targets: group1},
	{typ: "grpc", re: regexp.MustCompile(`grpc\.NewClient\(([^)]+)\)`), method: "dial", targets: group1},
	{typ: "grpc", re: regexp.MustCompile(`New(\w+)Client\(`), method: "client", targets: group1},
}

var mqPatterns = []*patternDef{
	{typ: "amqp", re: regexp.MustCompile(`amqp\.Dial\(([^)]+)\)`), method: "connect", targets: group1},
	{typ: "amqp", re: regexp.MustCompile(`channel\.Publish\(`), method: "publish", targets: literal("rabbitmq")},
	{typ: "amqp", re: regexp.MustCompile(`channel\.Consume\(`), method: "consume", targets: literal("rabbitmq")},
	{typ: "sns", re: regexp.MustCompile(`sns\.Publish\(`), method: "publish", targets: literal("SNS")},
	{typ: "sqs", re: regexp.MustCompile(`sqs\.(SendMessage|ReceiveMessage)\(`), method: "dynamic", targets: group1},
}

func group1(matches []string) string {
	if len(matches) > 1 {
		return strings.Trim(strings.TrimSpace(matches[1]), `"'`)
	}
	return ""
}

func group2(matches []string) string {
	if len(matches) > 2 {
		return strings.Trim(strings.TrimSpace(matches[2]), `"'`)
	}
	return ""
}

func literal(s string) func([]string) string {
	return func(_ []string) string { return s }
}

// DetectPatterns scans file content for cross-service communication patterns.
func (d *Detector) DetectPatterns(fileContent, filePath string) []CrossServiceCall {
	var calls []CrossServiceCall
	lines := strings.Split(fileContent, "\n")

	allPatterns := make([]*patternDef, 0, len(httpPatterns)+len(kafkaPatterns)+len(grpcPatterns)+len(mqPatterns))
	allPatterns = append(allPatterns, httpPatterns...)
	allPatterns = append(allPatterns, kafkaPatterns...)
	allPatterns = append(allPatterns, grpcPatterns...)
	allPatterns = append(allPatterns, mqPatterns...)

	for lineNum, line := range lines {
		for _, p := range allPatterns {
			matches := p.re.FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			method := p.method
			if method == "dynamic" && len(matches) > 1 {
				method = matches[1]
			}
			calls = append(calls, CrossServiceCall{
				Type:     p.typ,
				Target:   p.targets(matches),
				Method:   method,
				FilePath: filePath,
				Line:     lineNum + 1,
			})
		}
	}
	return calls
}
