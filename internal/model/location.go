package model

import (
	"math"
)

type ResolvedAddress struct {
	City     string
	District *string
}

type UserLocationPreview struct {
	PreviewID   string
	UserID      int64
	Address     ResolvedAddress
	Coordinates Coordinates
}

type UserLocation struct {
	UserID      int64
	City        string
	District    *string
	Coordinates Coordinates
}

type Coordinates struct {
	Latitude  float64
	Longitude float64
}

func (c Coordinates) Validate() error {
	if math.IsNaN(c.Latitude) || math.IsInf(c.Latitude, 0) ||
		c.Latitude < -90 || c.Latitude > 90 {
		return ErrInvalidCoordinates
	}

	if math.IsNaN(c.Longitude) || math.IsInf(c.Longitude, 0) ||
		c.Longitude < -180 || c.Longitude > 180 {
		return ErrInvalidCoordinates
	}

	return nil
}
