package cmd

import "net/http"

// writeSTSErrorRespone writes error headers
func writeSTSErrorResponse(w http.ResponseWriter, errorCode STSErrorCode) {
	stsError := getSTSError(errorCode)
	// Generate error response.
	stsErrorResponse := getSTSErrorResponse(stsError)
	encodedErrorResponse := encodeResponse(stsErrorResponse)
	writeResponse(w, stsError.HTTPStatusCode, encodedErrorResponse, mimeXML)
}
