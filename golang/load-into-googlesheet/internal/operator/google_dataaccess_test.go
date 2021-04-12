package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gorilla/mux"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/jwt"
	"google.golang.org/api/sheets/v4"
	"local.dev/sheetsLoader/internal/testutils"
)

type mockHttpClientFactoryGetter struct {
	called         bool
	ctx            context.Context
	mockHttpClient *http.Client
}

func (c *mockHttpClientFactoryGetter) get(config *jwt.Config) HttpClientFactory {
	c.called = true
	return c.httpClientFactory
}

func (c *mockHttpClientFactoryGetter) httpClientFactory(ctx context.Context) *http.Client {
	c.ctx = ctx
	return c.mockHttpClient
}

type mockConfigFactory struct {
	mockJWTConfig   *jwt.Config
	credentialsJSON string
	scopes          []string
}

func (params *mockConfigFactory) create(credentials []byte, scope ...string) (*jwt.Config, error) {
	params.credentialsJSON = string(credentials)
	params.scopes = scope
	return params.mockJWTConfig, nil
}

func TestNewGoogleConnection(t *testing.T) {
	// GIVEN
	credentialsJSON := `{"fake": "Credentials"}`
	credentials := bytes.NewBufferString(credentialsJSON).Bytes()
	scope := []string{
		"http://www.testscope.com/test1",
		"http://www.testscope.com/test2",
	}
	jwtConfig := jwt.Config{}
	httpClient := &http.Client{}
	httpClientFactoryGetter := &mockHttpClientFactoryGetter{mockHttpClient: httpClient}
	ctx := context.TODO()
	configFactory := &mockConfigFactory{
		mockJWTConfig: &jwtConfig,
	}
	context := GoogleContext{
		ConfigFactory:        configFactory.create,
		GetHttpClientFactory: httpClientFactoryGetter.get,
		Context:              ctx,
	}

	expected := map[string]interface{}{
		"credentialsJSON": credentialsJSON,
		"scope":           scope,
		"called":          true,
		"ctx":             ctx,
		"http.Client":     httpClient,
	}

	// WHEN
	cn, err := NewGoogleConnection(&context, credentials, scope...)
	if err != nil {
		t.Fatalf("TestGoogleDataAccess: Error %v", err)
	}
	actual := map[string]interface{}{
		"credentialsJSON": configFactory.credentialsJSON,
		"scope":           configFactory.scopes,
		"called":          httpClientFactoryGetter.called,
		"ctx":             httpClientFactoryGetter.ctx,
		"http.Client":     cn.(*connection).client,
	}
	// THEN
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf(
			"TestGoogleDataAccess: expected %v, actual %v",
			expected,
			actual,
		)
	}
}

func TestGoogleContext(t *testing.T) {
	// GIVEN
	jwtConfigFactoryPtr := testutils.GetFnPtr(google.JWTConfigFromJSON)
	getHttpClientFactoryFnPtr := testutils.GetFnPtr(getHttpClientFactory)
	ctx := context.Background()

	expected := map[string]interface{}{
		"ConfigFactory":        jwtConfigFactoryPtr,
		"GetHttpClientFactory": getHttpClientFactoryFnPtr,
		"ctx":                  ctx,
	}

	// WHEN
	googleContext := NewGoogleContext()

	actual := map[string]interface{}{
		"ConfigFactory": testutils.GetFnPtr(googleContext.ConfigFactory),

		"GetHttpClientFactory": testutils.GetFnPtr(googleContext.GetHttpClientFactory),
		"ctx":                  googleContext.Context,
	}

	// THEN
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf(
			"TestGoogleContext: expected: %v, actual %v",
			expected,
			actual,
		)
	}
}

type mockSpreadsheetsHandler struct {
	spreadsheetId string
	shts          []*sheets.Sheet
}

func spreadsheetIdFixture() string {
	return "spreadsheetId"
}

func sheetTitleFixture(index int64) string {
	return fmt.Sprintf("TAB%d", index)
}

func sheetIdFixture(index int64) int64 {
	return index
}

func sheetPropertiesFixture(index int64) *sheets.SheetProperties {
	properties := sheets.SheetProperties{
		Title:   sheetTitleFixture(index),
		SheetId: sheetIdFixture(index),
	}
	return &properties
}

func sheetsFixture() []*sheets.Sheet {
	return []*sheets.Sheet{
		&sheets.Sheet{
			Properties: sheetPropertiesFixture(1),
		},
		&sheets.Sheet{
			Properties: sheetPropertiesFixture(2),
		},
	}
}

func spreadsheetFixture() *sheets.Spreadsheet {
	shts := sheetsFixture()
	spreadsheet := &sheets.Spreadsheet{
		Sheets: shts,
	}
	return spreadsheet
}

func (h *mockSpreadsheetsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// https://sheets.googleapis.com/v4/spreadsheets/{spreadsheetId}
	vars := mux.Vars(r)
	spreadsheetId := vars["spreadsheetId"]
	if spreadsheetId == h.spreadsheetId {
		spreadsheet := spreadsheetFixture()
		response, _ := json.Marshal(spreadsheet)
		w.Write(response)
	}
}

func mockServerFixture() *httptest.Server {
	spreadsheetId := spreadsheetIdFixture()
	router := mux.NewRouter()
	shts := sheetsFixture()
	spreadsheetsHandler := &mockSpreadsheetsHandler{
		spreadsheetId: spreadsheetId,
		shts:          shts,
	}
	// https://sheets.googleapis.com/v4/spreadsheets/{spreadsheetId}
	router.Handle("/v4/spreadsheets/{spreadsheetId}", spreadsheetsHandler)
	mockServer := httptest.NewServer(router)
	return mockServer
}

func TestGoogleSheetOutput(t *testing.T) {
	// GIVEN
	mockServer := mockServerFixture()
	client := mockServer.Client()
	sheetNumber := int64(1)
	spreadsheetId := spreadsheetIdFixture()

	// WHEN
	srv, err := sheets.New(client)
	srv.BasePath = mockServer.URL
	if err != nil {
		t.Fatal(err)
	}
	spreadsheetsGetCall := srv.Spreadsheets.Get(spreadsheetId)
	spreadsheet, err := spreadsheetsGetCall.Context(context.Background()).Do()
	if err != nil {
		t.Fatalf("Unable to get spreadsheets: %v", err)
	}
	var sheetId int64
	sheetId = -1
	for _, s := range spreadsheet.Sheets {
		if s.Properties.Title == sheetTitleFixture(sheetNumber) {
			sheetId = s.Properties.SheetId
			break
		}
	}
	if sheetId == -1 {
		t.Fatalf(`Sheet ${sheetName} not found`)
	}
}
