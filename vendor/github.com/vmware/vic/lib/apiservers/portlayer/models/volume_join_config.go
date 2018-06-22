package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	strfmt "github.com/go-openapi/strfmt"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// VolumeJoinConfig volume join config
// swagger:model VolumeJoinConfig
type VolumeJoinConfig struct {

	// flags
	// Required: true
	Flags map[string]string `json:"Flags"`

	// handle
	// Required: true
	Handle string `json:"Handle"`

	// mount path
	// Required: true
	MountPath string `json:"MountPath"`
}

// Validate validates this volume join config
func (m *VolumeJoinConfig) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateFlags(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if err := m.validateHandle(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if err := m.validateMountPath(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *VolumeJoinConfig) validateFlags(formats strfmt.Registry) error {

	if swag.IsZero(m.Flags) { // not required
		return nil
	}

	return nil
}

func (m *VolumeJoinConfig) validateHandle(formats strfmt.Registry) error {

	if err := validate.RequiredString("Handle", "body", string(m.Handle)); err != nil {
		return err
	}

	return nil
}

func (m *VolumeJoinConfig) validateMountPath(formats strfmt.Registry) error {

	if err := validate.RequiredString("MountPath", "body", string(m.MountPath)); err != nil {
		return err
	}

	return nil
}
