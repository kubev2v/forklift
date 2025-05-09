// Code generated by go-swagger; DO NOT EDIT.

package application

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	"github.com/netapp/trident/storage_drivers/ontap/api/rest/models"
)

// ApplicationSnapshotDeleteReader is a Reader for the ApplicationSnapshotDelete structure.
type ApplicationSnapshotDeleteReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *ApplicationSnapshotDeleteReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 202:
		result := NewApplicationSnapshotDeleteAccepted()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewApplicationSnapshotDeleteDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewApplicationSnapshotDeleteAccepted creates a ApplicationSnapshotDeleteAccepted with default headers values
func NewApplicationSnapshotDeleteAccepted() *ApplicationSnapshotDeleteAccepted {
	return &ApplicationSnapshotDeleteAccepted{}
}

/*
ApplicationSnapshotDeleteAccepted describes a response with status code 202, with default header values.

Accepted
*/
type ApplicationSnapshotDeleteAccepted struct {
	Payload *models.JobLinkResponse
}

// IsSuccess returns true when this application snapshot delete accepted response has a 2xx status code
func (o *ApplicationSnapshotDeleteAccepted) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this application snapshot delete accepted response has a 3xx status code
func (o *ApplicationSnapshotDeleteAccepted) IsRedirect() bool {
	return false
}

// IsClientError returns true when this application snapshot delete accepted response has a 4xx status code
func (o *ApplicationSnapshotDeleteAccepted) IsClientError() bool {
	return false
}

// IsServerError returns true when this application snapshot delete accepted response has a 5xx status code
func (o *ApplicationSnapshotDeleteAccepted) IsServerError() bool {
	return false
}

// IsCode returns true when this application snapshot delete accepted response a status code equal to that given
func (o *ApplicationSnapshotDeleteAccepted) IsCode(code int) bool {
	return code == 202
}

func (o *ApplicationSnapshotDeleteAccepted) Error() string {
	return fmt.Sprintf("[DELETE /application/applications/{application.uuid}/snapshots/{uuid}][%d] applicationSnapshotDeleteAccepted  %+v", 202, o.Payload)
}

func (o *ApplicationSnapshotDeleteAccepted) String() string {
	return fmt.Sprintf("[DELETE /application/applications/{application.uuid}/snapshots/{uuid}][%d] applicationSnapshotDeleteAccepted  %+v", 202, o.Payload)
}

func (o *ApplicationSnapshotDeleteAccepted) GetPayload() *models.JobLinkResponse {
	return o.Payload
}

func (o *ApplicationSnapshotDeleteAccepted) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.JobLinkResponse)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewApplicationSnapshotDeleteDefault creates a ApplicationSnapshotDeleteDefault with default headers values
func NewApplicationSnapshotDeleteDefault(code int) *ApplicationSnapshotDeleteDefault {
	return &ApplicationSnapshotDeleteDefault{
		_statusCode: code,
	}
}

/*
ApplicationSnapshotDeleteDefault describes a response with status code -1, with default header values.

Error
*/
type ApplicationSnapshotDeleteDefault struct {
	_statusCode int

	Payload *models.ErrorResponse
}

// Code gets the status code for the application snapshot delete default response
func (o *ApplicationSnapshotDeleteDefault) Code() int {
	return o._statusCode
}

// IsSuccess returns true when this application snapshot delete default response has a 2xx status code
func (o *ApplicationSnapshotDeleteDefault) IsSuccess() bool {
	return o._statusCode/100 == 2
}

// IsRedirect returns true when this application snapshot delete default response has a 3xx status code
func (o *ApplicationSnapshotDeleteDefault) IsRedirect() bool {
	return o._statusCode/100 == 3
}

// IsClientError returns true when this application snapshot delete default response has a 4xx status code
func (o *ApplicationSnapshotDeleteDefault) IsClientError() bool {
	return o._statusCode/100 == 4
}

// IsServerError returns true when this application snapshot delete default response has a 5xx status code
func (o *ApplicationSnapshotDeleteDefault) IsServerError() bool {
	return o._statusCode/100 == 5
}

// IsCode returns true when this application snapshot delete default response a status code equal to that given
func (o *ApplicationSnapshotDeleteDefault) IsCode(code int) bool {
	return o._statusCode == code
}

func (o *ApplicationSnapshotDeleteDefault) Error() string {
	return fmt.Sprintf("[DELETE /application/applications/{application.uuid}/snapshots/{uuid}][%d] application_snapshot_delete default  %+v", o._statusCode, o.Payload)
}

func (o *ApplicationSnapshotDeleteDefault) String() string {
	return fmt.Sprintf("[DELETE /application/applications/{application.uuid}/snapshots/{uuid}][%d] application_snapshot_delete default  %+v", o._statusCode, o.Payload)
}

func (o *ApplicationSnapshotDeleteDefault) GetPayload() *models.ErrorResponse {
	return o.Payload
}

func (o *ApplicationSnapshotDeleteDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.ErrorResponse)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
