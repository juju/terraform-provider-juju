package juju

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"unicode"

	charmresources "github.com/juju/charm/v12/resource"
	jujuerrors "github.com/juju/errors"
	"github.com/juju/juju/api/base"
	apiapplication "github.com/juju/juju/api/client/application"
	apiresources "github.com/juju/juju/api/client/resources"
	jujuhttp "github.com/juju/juju/api/http"
	resourcecmd "github.com/juju/juju/cmd/juju/resource"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/rpc/params"
)

// newEndpointPath returns the API URL path for the identified resource.
func newEndpointPath(application string, name string) string {
	return fmt.Sprintf(HTTPEndpointPath, application, name)
}

// ResourceHttpClient returns a new Client for the given raw API caller.
func ResourceHttpClient(apiCaller base.APICallCloser) *HttpRequestClient {
	frontend, backend := base.NewClientFacade(apiCaller, "Resources")

	httpClient, err := apiCaller.HTTPClient()
	if err != nil {
		return nil
	}
	return &HttpRequestClient{
		ClientFacade: frontend,
		facade:       backend,
		httpClient:   httpClient,
	}
}

const (
	// ContentTypeRaw is the HTTP content-type value used for raw, unformatted content.
	ContentTypeRaw = "application/octet-stream"
)
const (
	// MediaTypeFormData is the media type for file uploads (see mime.FormatMediaType).
	MediaTypeFormData = "form-data"
	// QueryParamPendingID is the query parameter we use to send up the pending ID.
	QueryParamPendingID = "pendingid"
)

const (
	// HeaderContentType is the header name for the type of file upload.
	HeaderContentType = "Content-Type"
	// HeaderContentSha384 is the header name for the sha hash of a file upload.
	HeaderContentSha384 = "Content-Sha384"
	// HeaderContentLength is the header name for the length of a file upload.
	HeaderContentLength = "Content-Length"
	// HeaderContentDisposition is the header name for value that holds the filename.
	HeaderContentDisposition = "Content-Disposition"
)

const (
	// HTTPEndpointPath is the URL path, with substitutions, for a resource request.
	HTTPEndpointPath = "/applications/%s/resources/%s"
)

const FilenameParamForContentDispositionHeader = "filename"

// UploadRequest defines a single upload request.
type UploadRequest struct {
	// Application is the application ID.
	Application string

	// Name is the resource name.
	Name string

	// Filename is the name of the file as it exists on disk.
	Filename string

	// Size is the size of the uploaded data, in bytes.
	Size int64

	// Fingerprint is the fingerprint of the uploaded data.
	Fingerprint charmresources.Fingerprint

	// PendingID is the pending ID to associate with this upload, if any.
	PendingID string

	// Content is the content to upload.
	Content io.ReadSeeker
}

type HttpRequestClient struct {
	base.ClientFacade
	facade     base.FacadeCaller
	httpClient jujuhttp.HTTPDoer
}

type osFilesystem struct{}

func (osFilesystem) Create(name string) (*os.File, error) {
	return os.Create(name)
}

func (osFilesystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (osFilesystem) Open(name string) (modelcmd.ReadSeekCloser, error) {
	return os.Open(name)
}

func (osFilesystem) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (osFilesystem) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// isInt checks if strings consists from digits
// Used to detect resources which are given with revision number
func isInt(s string) bool {
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

// Upload sends the provided resource blob up to Juju.
func upload(appName, name, filename, pendingID string, reader io.ReadSeeker, resourceHttpClient *HttpRequestClient) error {
	uReq, err := apiresources.NewUploadRequest(appName, name, filename, reader)
	if err != nil {
		return jujuerrors.Trace(err)
	}
	if pendingID != "" {
		uReq.PendingID = pendingID
	}
	req, err := uReq.HTTPRequest()
	if err != nil {
		return jujuerrors.Trace(err)
	}
	var response params.UploadResult
	if err := resourceHttpClient.httpClient.Do(resourceHttpClient.facade.RawAPICaller().Context(), req, &response); err != nil {
		return jujuerrors.Trace(err)
	}

	return nil
}

// setFilename sets a name to the file.
func setFilename(filename string, req *http.Request) {
	filename = mime.BEncoding.Encode("utf-8", filename)

	disp := mime.FormatMediaType(
		MediaTypeFormData,
		map[string]string{FilenameParamForContentDispositionHeader: filename},
	)

	req.Header.Set(HeaderContentDisposition, disp)
}

// HTTPRequest generates a new HTTP request.
func (ur UploadRequest) HTTPRequest() (*http.Request, error) {
	urlStr := newEndpointPath(ur.Application, ur.Name)

	req, err := http.NewRequest(http.MethodPut, urlStr, ur.Content)
	if err != nil {
		return nil, jujuerrors.Trace(err)
	}

	req.Header.Set(HeaderContentType, ContentTypeRaw)
	req.Header.Set(HeaderContentSha384, ur.Fingerprint.String())
	req.Header.Set(HeaderContentLength, fmt.Sprint(ur.Size))
	setFilename(ur.Filename, req)

	req.ContentLength = ur.Size

	if ur.PendingID != "" {
		query := req.URL.Query()
		query.Set(QueryParamPendingID, ur.PendingID)
		req.URL.RawQuery = query.Encode()
	}

	return req, nil
}

// UploadExistingPendingResources uploads local resources. Used
// after DeployFromRepository, where the resources have been added
// to the controller.
func uploadExistingPendingResources(
	appName string,
	pendingResources []apiapplication.PendingResourceUpload,
	filesystem modelcmd.Filesystem,
	resourceHttpClient *HttpRequestClient) error {
	if pendingResources == nil {
		return nil
	}
	pendingID := ""

	for _, pendingResUpload := range pendingResources {
		t, typeParseErr := charmresources.ParseType(pendingResUpload.Type)
		if typeParseErr != nil {
			return jujuerrors.Annotatef(typeParseErr, "invalid type %v for pending resource %v",
				pendingResUpload.Type, pendingResUpload.Name)
		}

		r, openResErr := resourcecmd.OpenResource(pendingResUpload.Filename, t, filesystem.Open)
		if openResErr != nil {
			return jujuerrors.Annotatef(openResErr, "unable to open resource %v", pendingResUpload.Name)
		}
		uploadErr := upload(appName, pendingResUpload.Name, pendingResUpload.Filename, pendingID, r, resourceHttpClient)

		if uploadErr != nil {
			return jujuerrors.Trace(uploadErr)
		}
	}
	return nil
}
