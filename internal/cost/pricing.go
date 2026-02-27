package cost

// Rate is the per-million-token price in USD.
type Rate struct {
	InputPerMTok  float64
	OutputPerMTok float64
}

// Compute returns cost in USD for the given token counts.
func (r Rate) Compute(inputTokens, outputTokens int) float64 {
	return float64(inputTokens)/1_000_000*r.InputPerMTok +
		float64(outputTokens)/1_000_000*r.OutputPerMTok
}

// Pricing is a lookup table: provider -> model -> rate.
type Pricing struct {
	rates map[string]map[string]Rate
}

// Lookup returns the rate for a provider/model pair.
func (p *Pricing) Lookup(provider, model string) (Rate, bool) {
	models, ok := p.rates[provider]
	if !ok {
		return Rate{}, false
	}
	rate, ok := models[model]
	return rate, ok
}

// DefaultPricing returns a pricing table with well-known models.
// Prices in USD per million tokens. Updated manually.
func DefaultPricing() *Pricing {
	return &Pricing{rates: map[string]map[string]Rate{
		"anthropic": {
			"claude-sonnet-4":   {InputPerMTok: 3.0, OutputPerMTok: 15.0},
			"claude-sonnet-4-6": {InputPerMTok: 3.0, OutputPerMTok: 15.0},
			"claude-haiku-4-5":  {InputPerMTok: 0.80, OutputPerMTok: 4.0},
			"claude-opus-4":     {InputPerMTok: 15.0, OutputPerMTok: 75.0},
			"claude-opus-4-6":   {InputPerMTok: 15.0, OutputPerMTok: 75.0},
		},
		"openai": {
			"gpt-4o":       {InputPerMTok: 2.50, OutputPerMTok: 10.0},
			"gpt-4o-mini":  {InputPerMTok: 0.15, OutputPerMTok: 0.60},
			"gpt-4.1":      {InputPerMTok: 2.0, OutputPerMTok: 8.0},
			"gpt-4.1-mini": {InputPerMTok: 0.40, OutputPerMTok: 1.60},
			"gpt-4.1-nano": {InputPerMTok: 0.10, OutputPerMTok: 0.40},
			"o3":           {InputPerMTok: 2.0, OutputPerMTok: 8.0},
			"o4-mini":      {InputPerMTok: 1.10, OutputPerMTok: 4.40},
		},
		"openrouter": {},
	}}
}
