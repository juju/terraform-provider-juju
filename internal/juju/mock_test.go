// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/terraform-provider-juju/internal/juju (interfaces: SharedClient,ClientAPIClient,ApplicationAPIClient,ModelConfigAPIClient,ResourceAPIClient,SecretAPIClient)
//
// Generated by this command:
//
//	mockgen -package juju -destination mock_test.go github.com/juju/terraform-provider-juju/internal/juju SharedClient,ClientAPIClient,ApplicationAPIClient,ModelConfigAPIClient,ResourceAPIClient,SecretAPIClient
//

// Package juju is a generated GoMock package.
package juju

import (
	io "io"
	reflect "reflect"

	charm "github.com/juju/charm/v12"
	resource "github.com/juju/charm/v12/resource"
	api "github.com/juju/juju/api"
	application "github.com/juju/juju/api/client/application"
	client "github.com/juju/juju/api/client/client"
	resources "github.com/juju/juju/api/client/resources"
	secrets "github.com/juju/juju/api/client/secrets"
	charm0 "github.com/juju/juju/api/common/charm"
	constraints "github.com/juju/juju/core/constraints"
	model "github.com/juju/juju/core/model"
	resources0 "github.com/juju/juju/core/resources"
	secrets0 "github.com/juju/juju/core/secrets"
	params "github.com/juju/juju/rpc/params"
	names "github.com/juju/names/v5"
	gomock "go.uber.org/mock/gomock"
)

// MockSharedClient is a mock of SharedClient interface.
type MockSharedClient struct {
	ctrl     *gomock.Controller
	recorder *MockSharedClientMockRecorder
}

// MockSharedClientMockRecorder is the mock recorder for MockSharedClient.
type MockSharedClientMockRecorder struct {
	mock *MockSharedClient
}

// NewMockSharedClient creates a new mock instance.
func NewMockSharedClient(ctrl *gomock.Controller) *MockSharedClient {
	mock := &MockSharedClient{ctrl: ctrl}
	mock.recorder = &MockSharedClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSharedClient) EXPECT() *MockSharedClientMockRecorder {
	return m.recorder
}

// AddModel mocks base method.
func (m *MockSharedClient) AddModel(arg0, arg1 string, arg2 model.ModelType) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "AddModel", arg0, arg1, arg2)
}

// AddModel indicates an expected call of AddModel.
func (mr *MockSharedClientMockRecorder) AddModel(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddModel", reflect.TypeOf((*MockSharedClient)(nil).AddModel), arg0, arg1, arg2)
}

// Debugf mocks base method.
func (m *MockSharedClient) Debugf(arg0 string, arg1 ...map[string]any) {
	m.ctrl.T.Helper()
	varargs := []any{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Debugf", varargs...)
}

// Debugf indicates an expected call of Debugf.
func (mr *MockSharedClientMockRecorder) Debugf(arg0 any, arg1 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Debugf", reflect.TypeOf((*MockSharedClient)(nil).Debugf), varargs...)
}

// Errorf mocks base method.
func (m *MockSharedClient) Errorf(arg0 error, arg1 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Errorf", arg0, arg1)
}

// Errorf indicates an expected call of Errorf.
func (mr *MockSharedClientMockRecorder) Errorf(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Errorf", reflect.TypeOf((*MockSharedClient)(nil).Errorf), arg0, arg1)
}

// GetConnection mocks base method.
func (m *MockSharedClient) GetConnection(arg0 *string) (api.Connection, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetConnection", arg0)
	ret0, _ := ret[0].(api.Connection)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetConnection indicates an expected call of GetConnection.
func (mr *MockSharedClientMockRecorder) GetConnection(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetConnection", reflect.TypeOf((*MockSharedClient)(nil).GetConnection), arg0)
}

// JujuLogger mocks base method.
func (m *MockSharedClient) JujuLogger() *jujuLoggerShim {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "JujuLogger")
	ret0, _ := ret[0].(*jujuLoggerShim)
	return ret0
}

// JujuLogger indicates an expected call of JujuLogger.
func (mr *MockSharedClientMockRecorder) JujuLogger() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "JujuLogger", reflect.TypeOf((*MockSharedClient)(nil).JujuLogger))
}

// ModelType mocks base method.
func (m *MockSharedClient) ModelType(arg0 string) (model.ModelType, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ModelType", arg0)
	ret0, _ := ret[0].(model.ModelType)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ModelType indicates an expected call of ModelType.
func (mr *MockSharedClientMockRecorder) ModelType(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ModelType", reflect.TypeOf((*MockSharedClient)(nil).ModelType), arg0)
}

// ModelUUID mocks base method.
func (m *MockSharedClient) ModelUUID(arg0 string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ModelUUID", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ModelUUID indicates an expected call of ModelUUID.
func (mr *MockSharedClientMockRecorder) ModelUUID(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ModelUUID", reflect.TypeOf((*MockSharedClient)(nil).ModelUUID), arg0)
}

// RemoveModel mocks base method.
func (m *MockSharedClient) RemoveModel(arg0 string) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "RemoveModel", arg0)
}

// RemoveModel indicates an expected call of RemoveModel.
func (mr *MockSharedClientMockRecorder) RemoveModel(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveModel", reflect.TypeOf((*MockSharedClient)(nil).RemoveModel), arg0)
}

// Tracef mocks base method.
func (m *MockSharedClient) Tracef(arg0 string, arg1 ...map[string]any) {
	m.ctrl.T.Helper()
	varargs := []any{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Tracef", varargs...)
}

// Tracef indicates an expected call of Tracef.
func (mr *MockSharedClientMockRecorder) Tracef(arg0 any, arg1 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Tracef", reflect.TypeOf((*MockSharedClient)(nil).Tracef), varargs...)
}

// Warnf mocks base method.
func (m *MockSharedClient) Warnf(arg0 string, arg1 ...map[string]any) {
	m.ctrl.T.Helper()
	varargs := []any{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Warnf", varargs...)
}

// Warnf indicates an expected call of Warnf.
func (mr *MockSharedClientMockRecorder) Warnf(arg0 any, arg1 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Warnf", reflect.TypeOf((*MockSharedClient)(nil).Warnf), varargs...)
}

// MockClientAPIClient is a mock of ClientAPIClient interface.
type MockClientAPIClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientAPIClientMockRecorder
}

// MockClientAPIClientMockRecorder is the mock recorder for MockClientAPIClient.
type MockClientAPIClientMockRecorder struct {
	mock *MockClientAPIClient
}

// NewMockClientAPIClient creates a new mock instance.
func NewMockClientAPIClient(ctrl *gomock.Controller) *MockClientAPIClient {
	mock := &MockClientAPIClient{ctrl: ctrl}
	mock.recorder = &MockClientAPIClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClientAPIClient) EXPECT() *MockClientAPIClientMockRecorder {
	return m.recorder
}

// Status mocks base method.
func (m *MockClientAPIClient) Status(arg0 *client.StatusArgs) (*params.FullStatus, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Status", arg0)
	ret0, _ := ret[0].(*params.FullStatus)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Status indicates an expected call of Status.
func (mr *MockClientAPIClientMockRecorder) Status(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Status", reflect.TypeOf((*MockClientAPIClient)(nil).Status), arg0)
}

// MockApplicationAPIClient is a mock of ApplicationAPIClient interface.
type MockApplicationAPIClient struct {
	ctrl     *gomock.Controller
	recorder *MockApplicationAPIClientMockRecorder
}

// MockApplicationAPIClientMockRecorder is the mock recorder for MockApplicationAPIClient.
type MockApplicationAPIClientMockRecorder struct {
	mock *MockApplicationAPIClient
}

// NewMockApplicationAPIClient creates a new mock instance.
func NewMockApplicationAPIClient(ctrl *gomock.Controller) *MockApplicationAPIClient {
	mock := &MockApplicationAPIClient{ctrl: ctrl}
	mock.recorder = &MockApplicationAPIClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockApplicationAPIClient) EXPECT() *MockApplicationAPIClientMockRecorder {
	return m.recorder
}

// AddUnits mocks base method.
func (m *MockApplicationAPIClient) AddUnits(arg0 application.AddUnitsParams) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddUnits", arg0)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddUnits indicates an expected call of AddUnits.
func (mr *MockApplicationAPIClientMockRecorder) AddUnits(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddUnits", reflect.TypeOf((*MockApplicationAPIClient)(nil).AddUnits), arg0)
}

// ApplicationsInfo mocks base method.
func (m *MockApplicationAPIClient) ApplicationsInfo(arg0 []names.ApplicationTag) ([]params.ApplicationInfoResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ApplicationsInfo", arg0)
	ret0, _ := ret[0].([]params.ApplicationInfoResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ApplicationsInfo indicates an expected call of ApplicationsInfo.
func (mr *MockApplicationAPIClientMockRecorder) ApplicationsInfo(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ApplicationsInfo", reflect.TypeOf((*MockApplicationAPIClient)(nil).ApplicationsInfo), arg0)
}

// Deploy mocks base method.
func (m *MockApplicationAPIClient) Deploy(arg0 application.DeployArgs) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deploy", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Deploy indicates an expected call of Deploy.
func (mr *MockApplicationAPIClientMockRecorder) Deploy(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Deploy", reflect.TypeOf((*MockApplicationAPIClient)(nil).Deploy), arg0)
}

// DeployFromRepository mocks base method.
func (m *MockApplicationAPIClient) DeployFromRepository(arg0 application.DeployFromRepositoryArg) (application.DeployInfo, []application.PendingResourceUpload, []error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeployFromRepository", arg0)
	ret0, _ := ret[0].(application.DeployInfo)
	ret1, _ := ret[1].([]application.PendingResourceUpload)
	ret2, _ := ret[2].([]error)
	return ret0, ret1, ret2
}

// DeployFromRepository indicates an expected call of DeployFromRepository.
func (mr *MockApplicationAPIClientMockRecorder) DeployFromRepository(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeployFromRepository", reflect.TypeOf((*MockApplicationAPIClient)(nil).DeployFromRepository), arg0)
}

// DestroyApplications mocks base method.
func (m *MockApplicationAPIClient) DestroyApplications(arg0 application.DestroyApplicationsParams) ([]params.DestroyApplicationResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DestroyApplications", arg0)
	ret0, _ := ret[0].([]params.DestroyApplicationResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DestroyApplications indicates an expected call of DestroyApplications.
func (mr *MockApplicationAPIClientMockRecorder) DestroyApplications(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DestroyApplications", reflect.TypeOf((*MockApplicationAPIClient)(nil).DestroyApplications), arg0)
}

// DestroyUnits mocks base method.
func (m *MockApplicationAPIClient) DestroyUnits(arg0 application.DestroyUnitsParams) ([]params.DestroyUnitResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DestroyUnits", arg0)
	ret0, _ := ret[0].([]params.DestroyUnitResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DestroyUnits indicates an expected call of DestroyUnits.
func (mr *MockApplicationAPIClientMockRecorder) DestroyUnits(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DestroyUnits", reflect.TypeOf((*MockApplicationAPIClient)(nil).DestroyUnits), arg0)
}

// Expose mocks base method.
func (m *MockApplicationAPIClient) Expose(arg0 string, arg1 map[string]params.ExposedEndpoint) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Expose", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Expose indicates an expected call of Expose.
func (mr *MockApplicationAPIClientMockRecorder) Expose(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Expose", reflect.TypeOf((*MockApplicationAPIClient)(nil).Expose), arg0, arg1)
}

// Get mocks base method.
func (m *MockApplicationAPIClient) Get(arg0, arg1 string) (*params.ApplicationGetResults, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].(*params.ApplicationGetResults)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockApplicationAPIClientMockRecorder) Get(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockApplicationAPIClient)(nil).Get), arg0, arg1)
}

// GetCharmURLOrigin mocks base method.
func (m *MockApplicationAPIClient) GetCharmURLOrigin(arg0, arg1 string) (*charm.URL, charm0.Origin, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCharmURLOrigin", arg0, arg1)
	ret0, _ := ret[0].(*charm.URL)
	ret1, _ := ret[1].(charm0.Origin)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetCharmURLOrigin indicates an expected call of GetCharmURLOrigin.
func (mr *MockApplicationAPIClientMockRecorder) GetCharmURLOrigin(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCharmURLOrigin", reflect.TypeOf((*MockApplicationAPIClient)(nil).GetCharmURLOrigin), arg0, arg1)
}

// GetConstraints mocks base method.
func (m *MockApplicationAPIClient) GetConstraints(arg0 ...string) ([]constraints.Value, error) {
	m.ctrl.T.Helper()
	varargs := []any{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetConstraints", varargs...)
	ret0, _ := ret[0].([]constraints.Value)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetConstraints indicates an expected call of GetConstraints.
func (mr *MockApplicationAPIClientMockRecorder) GetConstraints(arg0 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetConstraints", reflect.TypeOf((*MockApplicationAPIClient)(nil).GetConstraints), arg0...)
}

// MergeBindings mocks base method.
func (m *MockApplicationAPIClient) MergeBindings(arg0 params.ApplicationMergeBindingsArgs) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MergeBindings", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// MergeBindings indicates an expected call of MergeBindings.
func (mr *MockApplicationAPIClientMockRecorder) MergeBindings(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MergeBindings", reflect.TypeOf((*MockApplicationAPIClient)(nil).MergeBindings), arg0)
}

// ScaleApplication mocks base method.
func (m *MockApplicationAPIClient) ScaleApplication(arg0 application.ScaleApplicationParams) (params.ScaleApplicationResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ScaleApplication", arg0)
	ret0, _ := ret[0].(params.ScaleApplicationResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ScaleApplication indicates an expected call of ScaleApplication.
func (mr *MockApplicationAPIClientMockRecorder) ScaleApplication(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ScaleApplication", reflect.TypeOf((*MockApplicationAPIClient)(nil).ScaleApplication), arg0)
}

// SetCharm mocks base method.
func (m *MockApplicationAPIClient) SetCharm(arg0 string, arg1 application.SetCharmConfig) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetCharm", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetCharm indicates an expected call of SetCharm.
func (mr *MockApplicationAPIClientMockRecorder) SetCharm(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetCharm", reflect.TypeOf((*MockApplicationAPIClient)(nil).SetCharm), arg0, arg1)
}

// SetConfig mocks base method.
func (m *MockApplicationAPIClient) SetConfig(arg0, arg1, arg2 string, arg3 map[string]string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetConfig", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetConfig indicates an expected call of SetConfig.
func (mr *MockApplicationAPIClientMockRecorder) SetConfig(arg0, arg1, arg2, arg3 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetConfig", reflect.TypeOf((*MockApplicationAPIClient)(nil).SetConfig), arg0, arg1, arg2, arg3)
}

// SetConstraints mocks base method.
func (m *MockApplicationAPIClient) SetConstraints(arg0 string, arg1 constraints.Value) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetConstraints", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetConstraints indicates an expected call of SetConstraints.
func (mr *MockApplicationAPIClientMockRecorder) SetConstraints(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetConstraints", reflect.TypeOf((*MockApplicationAPIClient)(nil).SetConstraints), arg0, arg1)
}

// Unexpose mocks base method.
func (m *MockApplicationAPIClient) Unexpose(arg0 string, arg1 []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Unexpose", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Unexpose indicates an expected call of Unexpose.
func (mr *MockApplicationAPIClientMockRecorder) Unexpose(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Unexpose", reflect.TypeOf((*MockApplicationAPIClient)(nil).Unexpose), arg0, arg1)
}

// MockModelConfigAPIClient is a mock of ModelConfigAPIClient interface.
type MockModelConfigAPIClient struct {
	ctrl     *gomock.Controller
	recorder *MockModelConfigAPIClientMockRecorder
}

// MockModelConfigAPIClientMockRecorder is the mock recorder for MockModelConfigAPIClient.
type MockModelConfigAPIClientMockRecorder struct {
	mock *MockModelConfigAPIClient
}

// NewMockModelConfigAPIClient creates a new mock instance.
func NewMockModelConfigAPIClient(ctrl *gomock.Controller) *MockModelConfigAPIClient {
	mock := &MockModelConfigAPIClient{ctrl: ctrl}
	mock.recorder = &MockModelConfigAPIClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockModelConfigAPIClient) EXPECT() *MockModelConfigAPIClientMockRecorder {
	return m.recorder
}

// ModelGet mocks base method.
func (m *MockModelConfigAPIClient) ModelGet() (map[string]any, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ModelGet")
	ret0, _ := ret[0].(map[string]any)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ModelGet indicates an expected call of ModelGet.
func (mr *MockModelConfigAPIClientMockRecorder) ModelGet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ModelGet", reflect.TypeOf((*MockModelConfigAPIClient)(nil).ModelGet))
}

// MockResourceAPIClient is a mock of ResourceAPIClient interface.
type MockResourceAPIClient struct {
	ctrl     *gomock.Controller
	recorder *MockResourceAPIClientMockRecorder
}

// MockResourceAPIClientMockRecorder is the mock recorder for MockResourceAPIClient.
type MockResourceAPIClientMockRecorder struct {
	mock *MockResourceAPIClient
}

// NewMockResourceAPIClient creates a new mock instance.
func NewMockResourceAPIClient(ctrl *gomock.Controller) *MockResourceAPIClient {
	mock := &MockResourceAPIClient{ctrl: ctrl}
	mock.recorder = &MockResourceAPIClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockResourceAPIClient) EXPECT() *MockResourceAPIClientMockRecorder {
	return m.recorder
}

// AddPendingResources mocks base method.
func (m *MockResourceAPIClient) AddPendingResources(arg0 resources.AddPendingResourcesArgs) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddPendingResources", arg0)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddPendingResources indicates an expected call of AddPendingResources.
func (mr *MockResourceAPIClientMockRecorder) AddPendingResources(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddPendingResources", reflect.TypeOf((*MockResourceAPIClient)(nil).AddPendingResources), arg0)
}

// ListResources mocks base method.
func (m *MockResourceAPIClient) ListResources(arg0 []string) ([]resources0.ApplicationResources, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListResources", arg0)
	ret0, _ := ret[0].([]resources0.ApplicationResources)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListResources indicates an expected call of ListResources.
func (mr *MockResourceAPIClientMockRecorder) ListResources(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListResources", reflect.TypeOf((*MockResourceAPIClient)(nil).ListResources), arg0)
}

// Upload mocks base method.
func (m *MockResourceAPIClient) Upload(arg0, arg1, arg2, arg3 string, arg4 io.ReadSeeker) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Upload", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// Upload indicates an expected call of Upload.
func (mr *MockResourceAPIClientMockRecorder) Upload(arg0, arg1, arg2, arg3, arg4 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Upload", reflect.TypeOf((*MockResourceAPIClient)(nil).Upload), arg0, arg1, arg2, arg3, arg4)
}

// UploadPendingResource mocks base method.
func (m *MockResourceAPIClient) UploadPendingResource(arg0 string, arg1 resource.Resource, arg2 string, arg3 io.ReadSeeker) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UploadPendingResource", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UploadPendingResource indicates an expected call of UploadPendingResource.
func (mr *MockResourceAPIClientMockRecorder) UploadPendingResource(arg0, arg1, arg2, arg3 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UploadPendingResource", reflect.TypeOf((*MockResourceAPIClient)(nil).UploadPendingResource), arg0, arg1, arg2, arg3)
}

// MockSecretAPIClient is a mock of SecretAPIClient interface.
type MockSecretAPIClient struct {
	ctrl     *gomock.Controller
	recorder *MockSecretAPIClientMockRecorder
}

// MockSecretAPIClientMockRecorder is the mock recorder for MockSecretAPIClient.
type MockSecretAPIClientMockRecorder struct {
	mock *MockSecretAPIClient
}

// NewMockSecretAPIClient creates a new mock instance.
func NewMockSecretAPIClient(ctrl *gomock.Controller) *MockSecretAPIClient {
	mock := &MockSecretAPIClient{ctrl: ctrl}
	mock.recorder = &MockSecretAPIClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSecretAPIClient) EXPECT() *MockSecretAPIClientMockRecorder {
	return m.recorder
}

// CreateSecret mocks base method.
func (m *MockSecretAPIClient) CreateSecret(arg0, arg1 string, arg2 map[string]string) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateSecret", arg0, arg1, arg2)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateSecret indicates an expected call of CreateSecret.
func (mr *MockSecretAPIClientMockRecorder) CreateSecret(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateSecret", reflect.TypeOf((*MockSecretAPIClient)(nil).CreateSecret), arg0, arg1, arg2)
}

// GrantSecret mocks base method.
func (m *MockSecretAPIClient) GrantSecret(arg0 *secrets0.URI, arg1 string, arg2 []string) ([]error, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GrantSecret", arg0, arg1, arg2)
	ret0, _ := ret[0].([]error)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GrantSecret indicates an expected call of GrantSecret.
func (mr *MockSecretAPIClientMockRecorder) GrantSecret(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GrantSecret", reflect.TypeOf((*MockSecretAPIClient)(nil).GrantSecret), arg0, arg1, arg2)
}

// ListSecrets mocks base method.
func (m *MockSecretAPIClient) ListSecrets(arg0 bool, arg1 secrets0.Filter) ([]secrets.SecretDetails, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListSecrets", arg0, arg1)
	ret0, _ := ret[0].([]secrets.SecretDetails)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListSecrets indicates an expected call of ListSecrets.
func (mr *MockSecretAPIClientMockRecorder) ListSecrets(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListSecrets", reflect.TypeOf((*MockSecretAPIClient)(nil).ListSecrets), arg0, arg1)
}

// RemoveSecret mocks base method.
func (m *MockSecretAPIClient) RemoveSecret(arg0 *secrets0.URI, arg1 string, arg2 *int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveSecret", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveSecret indicates an expected call of RemoveSecret.
func (mr *MockSecretAPIClientMockRecorder) RemoveSecret(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveSecret", reflect.TypeOf((*MockSecretAPIClient)(nil).RemoveSecret), arg0, arg1, arg2)
}

// RevokeSecret mocks base method.
func (m *MockSecretAPIClient) RevokeSecret(arg0 *secrets0.URI, arg1 string, arg2 []string) ([]error, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RevokeSecret", arg0, arg1, arg2)
	ret0, _ := ret[0].([]error)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RevokeSecret indicates an expected call of RevokeSecret.
func (mr *MockSecretAPIClientMockRecorder) RevokeSecret(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RevokeSecret", reflect.TypeOf((*MockSecretAPIClient)(nil).RevokeSecret), arg0, arg1, arg2)
}

// UpdateSecret mocks base method.
func (m *MockSecretAPIClient) UpdateSecret(arg0 *secrets0.URI, arg1 string, arg2 *bool, arg3, arg4 string, arg5 map[string]string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateSecret", arg0, arg1, arg2, arg3, arg4, arg5)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateSecret indicates an expected call of UpdateSecret.
func (mr *MockSecretAPIClientMockRecorder) UpdateSecret(arg0, arg1, arg2, arg3, arg4, arg5 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateSecret", reflect.TypeOf((*MockSecretAPIClient)(nil).UpdateSecret), arg0, arg1, arg2, arg3, arg4, arg5)
}
