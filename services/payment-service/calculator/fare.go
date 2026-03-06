package calculator

// FareConfig holds pricing for each vehicle type
type FareConfig struct {
	BaseFare      float64
	PerKmRate     float64
	PerMinuteRate float64
	BookingFee    float64
	MinimumFare   float64
	DriverPercent float64 // Percentage of fare that goes to driver
}

var fareConfigs = map[string]FareConfig{
	"ECONOMY": {
		BaseFare:      2.50,
		PerKmRate:     1.50,
		PerMinuteRate: 0.20,
		BookingFee:    1.00,
		MinimumFare:   5.00,
		DriverPercent: 0.75,
	},
	"COMFORT": {
		BaseFare:      3.50,
		PerKmRate:     2.00,
		PerMinuteRate: 0.30,
		BookingFee:    1.50,
		MinimumFare:   7.00,
		DriverPercent: 0.75,
	},
	"PREMIUM": {
		BaseFare:      5.00,
		PerKmRate:     3.00,
		PerMinuteRate: 0.50,
		BookingFee:    2.00,
		MinimumFare:   10.00,
		DriverPercent: 0.80,
	},
	"SUV": {
		BaseFare:      4.00,
		PerKmRate:     2.50,
		PerMinuteRate: 0.40,
		BookingFee:    1.50,
		MinimumFare:   8.00,
		DriverPercent: 0.75,
	},
	"XL": {
		BaseFare:      4.50,
		PerKmRate:     2.75,
		PerMinuteRate: 0.45,
		BookingFee:    2.00,
		MinimumFare:   9.00,
		DriverPercent: 0.75,
	},
}

type FareBreakdown struct {
	BaseFare     float64
	DistanceFare float64
	TimeFare     float64
	SurgeCharge  float64
	BookingFee   float64
	TotalFare    float64
	DriverPayout float64
	PlatformFee  float64
	Currency     string
}

func CalculateFare(distanceKm, durationMinutes float64, vehicleType string, surgeMultiplier float64) *FareBreakdown {
	config, exists := fareConfigs[vehicleType]
	if !exists {
		config = fareConfigs["ECONOMY"]
	}

	if surgeMultiplier < 1.0 {
		surgeMultiplier = 1.0
	}

	baseFare := config.BaseFare
	distanceFare := distanceKm * config.PerKmRate
	timeFare := durationMinutes * config.PerMinuteRate
	subtotal := baseFare + distanceFare + timeFare
	surgeCharge := subtotal * (surgeMultiplier - 1.0)
	bookingFee := config.BookingFee

	totalFare := subtotal + surgeCharge + bookingFee

	// Apply minimum fare
	if totalFare < config.MinimumFare {
		totalFare = config.MinimumFare
	}

	driverPayout := (totalFare - bookingFee) * config.DriverPercent
	platformFee := totalFare - driverPayout

	return &FareBreakdown{
		BaseFare:     baseFare,
		DistanceFare: distanceFare,
		TimeFare:     timeFare,
		SurgeCharge:  surgeCharge,
		BookingFee:   bookingFee,
		TotalFare:    totalFare,
		DriverPayout: driverPayout,
		PlatformFee:  platformFee,
		Currency:     "USD",
	}
}
