package helpers

import (
	"encoding/json"
	"io"
	"net/http"

	admissionapi "k8s.io/api/admission/v1beta1"
	admissionctl "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SendResponse Send the AdmissionReview.
func SendResponse(w io.Writer, resp admissionctl.Response) {

	encoder := json.NewEncoder(w)
	responseAdmissionReview := admissionapi.AdmissionReview{
		Response: &resp.AdmissionResponse,
	}
	err := encoder.Encode(responseAdmissionReview)
	if err != nil {
		SendResponse(w, admissionctl.Errored(http.StatusInternalServerError, err))
	}
}
