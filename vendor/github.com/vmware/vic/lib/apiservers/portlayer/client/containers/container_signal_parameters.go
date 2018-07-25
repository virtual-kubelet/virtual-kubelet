package containers

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/swag"

	strfmt "github.com/go-openapi/strfmt"
)

// NewContainerSignalParams creates a new ContainerSignalParams object
// with the default values initialized.
func NewContainerSignalParams() *ContainerSignalParams {
	var ()
	return &ContainerSignalParams{

		timeout: cr.DefaultTimeout,
	}
}

// NewContainerSignalParamsWithTimeout creates a new ContainerSignalParams object
// with the default values initialized, and the ability to set a timeout on a request
func NewContainerSignalParamsWithTimeout(timeout time.Duration) *ContainerSignalParams {
	var ()
	return &ContainerSignalParams{

		timeout: timeout,
	}
}

// NewContainerSignalParamsWithContext creates a new ContainerSignalParams object
// with the default values initialized, and the ability to set a context for a request
func NewContainerSignalParamsWithContext(ctx context.Context) *ContainerSignalParams {
	var ()
	return &ContainerSignalParams{

		Context: ctx,
	}
}

// NewContainerSignalParamsWithHTTPClient creates a new ContainerSignalParams object
// with the default values initialized, and the ability to set a custom HTTPClient for a request
func NewContainerSignalParamsWithHTTPClient(client *http.Client) *ContainerSignalParams {
	var ()
	return &ContainerSignalParams{
		HTTPClient: client,
	}
}

/*ContainerSignalParams contains all the parameters to send to the API endpoint
for the container signal operation typically these are written to a http.Request
*/
type ContainerSignalParams struct {

	/*OpID*/
	OpID *string
	/*ID*/
	ID string
	/*Signal*/
	Signal int64

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithTimeout adds the timeout to the container signal params
func (o *ContainerSignalParams) WithTimeout(timeout time.Duration) *ContainerSignalParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the container signal params
func (o *ContainerSignalParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the container signal params
func (o *ContainerSignalParams) WithContext(ctx context.Context) *ContainerSignalParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the container signal params
func (o *ContainerSignalParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the container signal params
func (o *ContainerSignalParams) WithHTTPClient(client *http.Client) *ContainerSignalParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the container signal params
func (o *ContainerSignalParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithOpID adds the opID to the container signal params
func (o *ContainerSignalParams) WithOpID(opID *string) *ContainerSignalParams {
	o.SetOpID(opID)
	return o
}

// SetOpID adds the opId to the container signal params
func (o *ContainerSignalParams) SetOpID(opID *string) {
	o.OpID = opID
}

// WithID adds the id to the container signal params
func (o *ContainerSignalParams) WithID(id string) *ContainerSignalParams {
	o.SetID(id)
	return o
}

// SetID adds the id to the container signal params
func (o *ContainerSignalParams) SetID(id string) {
	o.ID = id
}

// WithSignal adds the signal to the container signal params
func (o *ContainerSignalParams) WithSignal(signal int64) *ContainerSignalParams {
	o.SetSignal(signal)
	return o
}

// SetSignal adds the signal to the container signal params
func (o *ContainerSignalParams) SetSignal(signal int64) {
	o.Signal = signal
}

// WriteToRequest writes these params to a swagger request
func (o *ContainerSignalParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	r.SetTimeout(o.timeout)
	var res []error

	if o.OpID != nil {

		// header param Op-ID
		if err := r.SetHeaderParam("Op-ID", *o.OpID); err != nil {
			return err
		}

	}

	// path param id
	if err := r.SetPathParam("id", o.ID); err != nil {
		return err
	}

	// query param signal
	qrSignal := o.Signal
	qSignal := swag.FormatInt64(qrSignal)
	if qSignal != "" {
		if err := r.SetQueryParam("signal", qSignal); err != nil {
			return err
		}
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
