package containers

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"

	strfmt "github.com/go-openapi/strfmt"

	"github.com/vmware/vic/lib/apiservers/portlayer/models"
)

// StateChangeReader is a Reader for the StateChange structure.
type StateChangeReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *StateChangeReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {

	case 200:
		result := NewStateChangeOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil

	case 404:
		result := NewStateChangeNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result

	default:
		result := NewStateChangeDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewStateChangeOK creates a StateChangeOK with default headers values
func NewStateChangeOK() *StateChangeOK {
	return &StateChangeOK{}
}

/*StateChangeOK handles this case with default header values.

OK
*/
type StateChangeOK struct {
	Payload string
}

func (o *StateChangeOK) Error() string {
	return fmt.Sprintf("[PUT /containers/{handle}/state][%d] stateChangeOK  %+v", 200, o.Payload)
}

func (o *StateChangeOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewStateChangeNotFound creates a StateChangeNotFound with default headers values
func NewStateChangeNotFound() *StateChangeNotFound {
	return &StateChangeNotFound{}
}

/*StateChangeNotFound handles this case with default header values.

not found
*/
type StateChangeNotFound struct {
	Payload *models.Error
}

func (o *StateChangeNotFound) Error() string {
	return fmt.Sprintf("[PUT /containers/{handle}/state][%d] stateChangeNotFound  %+v", 404, o.Payload)
}

func (o *StateChangeNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewStateChangeDefault creates a StateChangeDefault with default headers values
func NewStateChangeDefault(code int) *StateChangeDefault {
	return &StateChangeDefault{
		_statusCode: code,
	}
}

/*StateChangeDefault handles this case with default header values.

Error
*/
type StateChangeDefault struct {
	_statusCode int

	Payload *models.Error
}

// Code gets the status code for the state change default response
func (o *StateChangeDefault) Code() int {
	return o._statusCode
}

func (o *StateChangeDefault) Error() string {
	return fmt.Sprintf("[PUT /containers/{handle}/state][%d] StateChange default  %+v", o._statusCode, o.Payload)
}

func (o *StateChangeDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.Error)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
