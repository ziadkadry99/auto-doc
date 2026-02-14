package llm

// modelPricing holds per-model pricing in USD per 1M tokens.
type modelPricing struct {
	InputPerMillion  float64
	OutputPerMillion float64
}

// priceTable maps model identifiers to their pricing.
var priceTable = map[string]modelPricing{
	// Anthropic models
	"claude-sonnet-4-5-20250929": {InputPerMillion: 3.00, OutputPerMillion: 15.00},
	"claude-haiku-4-5-20251001":  {InputPerMillion: 0.80, OutputPerMillion: 4.00},
	"claude-opus-4-6":            {InputPerMillion: 15.00, OutputPerMillion: 75.00},

	// OpenAI models
	"gpt-4o":      {InputPerMillion: 2.50, OutputPerMillion: 10.00},
	"gpt-4o-mini": {InputPerMillion: 0.15, OutputPerMillion: 0.60},

	// Google models
	"gemini-2.0-flash": {InputPerMillion: 0.10, OutputPerMillion: 0.40},
	"gemini-1.5-pro":   {InputPerMillion: 1.25, OutputPerMillion: 5.00},
}

// EstimateCost returns the estimated cost in USD for the given model and token counts.
// Returns 0 if the model is not found in the price table.
func EstimateCost(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := priceTable[model]
	if !ok {
		return 0
	}

	inputCost := float64(inputTokens) / 1_000_000.0 * pricing.InputPerMillion
	outputCost := float64(outputTokens) / 1_000_000.0 * pricing.OutputPerMillion
	return inputCost + outputCost
}

// EstimateTokens provides a rough token count estimation for the given text.
// Uses the approximation of 1 token per 4 characters.
func EstimateTokens(text string) int {
	n := len(text) / 4
	if n == 0 && len(text) > 0 {
		return 1
	}
	return n
}
