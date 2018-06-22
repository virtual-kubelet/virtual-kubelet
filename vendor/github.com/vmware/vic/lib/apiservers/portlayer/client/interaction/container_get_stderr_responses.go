package interaction

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"

	strfmt "github.com/go-openapi/strfmt"
)

// ContainerGetStderrReader is a Reader for the ContainerGetStderr structure.
type ContainerGetStderrReader struct {
	formats strfmt.Registry
	writer  io.Writer
}

// ReadResponse reads a server response into the received o.
func (o *ContainerGetStderrReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {

	case 200:
		result := NewContainerGetStderrOK(o.writer)
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil

	case 404:
		result := NewContainerGetStderrNotFound()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result

	case 500:
		result := NewContainerGetStderrInternalServerError()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result

	default:
		return nil, runtime.NewAPIError("unknown error", response, response.Code())
	}
}

// NewContainerGetStderrOK creates a ContainerGetStderrOK with default headers values
func NewContainerGetStderrOK(writer io.Writer) *ContainerGetStderrOK {
	return &ContainerGetStderrOK{
		Payload: writer,
	}
}

/*ContainerGetStderrOK handles this case with default header values.

OK
*/
type ContainerGetStderrOK struct {
	Payload io.Writer
}

func (o *ContainerGetStderrOK) Error() string {
	return fmt.Sprintf("[GET /interaction/{id}/stderr][%d] containerGetStderrOK  %+v", 200, o.Payload)
}

func (o *ContainerGetStderrOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewContainerGetStderrNotFound creates a ContainerGetStderrNotFound with default headers values
func NewContainerGetStderrNotFound() *ContainerGetStderrNotFound {
	return &ContainerGetStderrNotFound{}
}

/*ContainerGetStderrNotFound handles this case with default header values.

Container not found
*/
type ContainerGetStderrNotFound struct {
}

func (o *ContainerGetStderrNotFound) Error() string {
	return fmt.Sprintf("[GET /interaction/{id}/stderr][%d] containerGetStderrNotFound ", 404)
}

func (o *ContainerGetStderrNotFound) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}

// NewContainerGetStderrInternalServerError creates a ContainerGetStderrInternalServerError with default headers values
func NewContainerGetStderrInternalServerError() *ContainerGetStderrInternalServerError {
	return &ContainerGetStderrInternalServerError{}
}

/*ContainerGetStderrInternalServerError handles this case with default header values.

Failed to get stderr
*/
type ContainerGetStderrInternalServerError struct {
}

func (o *ContainerGetStderrInternalServerError) Error() string {
	return fmt.Sprintf("[GET /interaction/{id}/stderr][%d] containerGetStderrInternalServerError ", 500)
}

func (o *ContainerGetStderrInternalServerError) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	return nil
}
