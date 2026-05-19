package domain

// Static exchange rates to USD as of Dec 2024.
var ExchangeRates = map[string]float64{
	"USD": 1.0,
	"AED": 0.2723,
	"EUR": 1.0812,
	"GBP": 1.2650,
}

// ToUSD converts an amount from the given currency to USD.
func ToUSD(amount float64, currency string) float64 {
	rate, ok := ExchangeRates[currency]
	if !ok {
		return amount
	}
	return amount * rate
}

// SupportedCurrencies returns the list of currencies we handle.
func SupportedCurrencies() []string {
	return []string{"USD", "AED", "EUR", "GBP"}
}
