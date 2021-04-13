package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

type mockFactory struct {
	mockJWTConfig              *jwt.Config
	credentialsJSON            string
	scopes                     []string
	getHttpClientFactoryCalled bool
	ctx                        context.Context
	mockHttpClient             *http.Client
	mockSheetsService          *sheets.Service
}

func (params *mockFactory) createConfig(credentials []byte, scope ...string) (*jwt.Config, error) {
	params.credentialsJSON = string(credentials)
	params.scopes = scope
	return params.mockJWTConfig, nil
}

func (c *mockFactory) getHttpClientFactory(config *jwt.Config) HttpClientFactory {
	c.getHttpClientFactoryCalled = true
	return c.httpClientFactory
}

func (c *mockFactory) httpClientFactory(ctx context.Context) *http.Client {
	c.ctx = ctx
	return c.mockHttpClient
}

func (c *mockFactory) createService(client *http.Client) (*sheets.Service, error) {
	if client != c.mockHttpClient {
		return nil, errors.New("Invalid http client parameter.")
	}
	return c.mockSheetsService, nil
}

func TestNewGoogleSheetsConnection(t *testing.T) {
	// GIVEN
	credentialsJSON := `{"fake": "Credentials"}`
	credentials := bytes.NewBufferString(credentialsJSON).Bytes()
	scope := []string{
		"http://www.testscope.com/test1",
		"http://www.testscope.com/test2",
	}
	jwtConfig := jwt.Config{}
	httpClient := &http.Client{}
	sheetsService := &sheets.Service{}
	ctx := context.TODO()
	factory := &mockFactory{
		mockJWTConfig:     &jwtConfig,
		mockHttpClient:    httpClient,
		mockSheetsService: sheetsService,
	}
	context := GoogleContext{
		ConfigFactory:        factory.createConfig,
		GetHttpClientFactory: factory.getHttpClientFactory,
		ServiceFactory:       factory.createService,
		Context:              ctx,
	}

	expected := map[string]interface{}{
		"credentialsJSON":            credentialsJSON,
		"scope":                      scope,
		"getHttpClientFactoryCalled": true,
		"ctx":                        ctx,
		"sheets.Service":             sheetsService,
	}

	// WHEN
	cn, err := NewGoogleSheetsConnection(&context, credentials, scope...)
	if err != nil {
		t.Fatalf("TestGoogleDataAccess: Error %v", err)
	}
	actual := map[string]interface{}{
		"credentialsJSON":            factory.credentialsJSON,
		"scope":                      factory.scopes,
		"getHttpClientFactoryCalled": factory.getHttpClientFactoryCalled,
		"ctx":                        factory.ctx,
		"sheets.Service":             cn.(*connection).srv,
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

func dataFixture() string {
	return "Hello|World"
}

func gridCoordinateFixture(index int64) *sheets.GridCoordinate {
	sheetId := sheetIdFixture(index)
	return &sheets.GridCoordinate{
		SheetId:     sheetId,
		ColumnIndex: 0,
		RowIndex:    0,
	}
}

type mockSpreadsheetsHandler struct{}

func (h *mockSpreadsheetsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// https://sheets.googleapis.com/v4/spreadsheets/{spreadsheetId}
	vars := mux.Vars(r)
	spreadsheetId := vars["spreadsheetId"]
	if spreadsheetId == spreadsheetIdFixture() {
		spreadsheet := spreadsheetFixture()
		json.NewEncoder(w).Encode(spreadsheet)
	}
}

type mockBatchUpdateHandler struct {
	request sheets.BatchUpdateSpreadsheetRequest
}

func (h *mockBatchUpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// https://sheets.googleapis.com/v4/spreadsheets/{spreadsheetId}:batchUpda
	vars := mux.Vars(r)
	spreadsheetId := vars["spreadsheetId"]
	if spreadsheetId == spreadsheetIdFixture() {
		err := json.NewDecoder(r.Body).Decode(&h.request)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			batchUpdateSpreadsheetResponse := sheets.BatchUpdateSpreadsheetResponse{
				SpreadsheetId: spreadsheetId,
			}
			json.NewEncoder(w).Encode(batchUpdateSpreadsheetResponse)
		}
	}
}

func mockServerFixture() *httptest.Server {
	router := mux.NewRouter()
	// https://sheets.googleapis.com/v4/spreadsheets/{spreadsheetId}
	spreadsheetsHandler := &mockSpreadsheetsHandler{}
	router.Handle(
		"/v4/spreadsheets/{spreadsheetId}", spreadsheetsHandler,
	).Methods("GET")
	batchUpdateHandler := &mockBatchUpdateHandler{}
	// https://sheets.googleapis.com/v4/spreadsheets/{spreadsheetId}:batchUpdate
	router.Handle(
		"/v4/spreadsheets/{spreadsheetId}:batchUpdate", batchUpdateHandler,
	).Methods("POST")
	mockServer := httptest.NewServer(router)
	return mockServer
}

// type mockGoogleConnection struct {
// 	client *http.Client
// }

// func (c *mockGoogleConnection) Get(impl interface{}) {
// 	*(impl.(**http.Client)) = c.client
// }

// func TestGoogleSheetOutput(t *testing.T) {
// 	// GIVEN
// 	mockServer := mockServerFixture()
// 	httpClient := mockServer.Client()
// 	sheetNumber := int64(1)
// 	spreadsheetId := spreadsheetIdFixture()
// 	startCellRef := "Test!B2"
// 	data := "Hello|World"
// 	output := NewGoogleSheetOutput(startCellRef)

// 	// WHEN
// 	output.Write()
// 	srv, err := sheets.New(client)
// 	// Update the basepath to use the mock url.
// 	srv.BasePath = mockServer.URL
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	spreadsheetsGetCall := srv.Spreadsheets.Get(spreadsheetId)
// 	spreadsheet, err := spreadsheetsGetCall.Context(context.Background()).Do()
// 	if err != nil {
// 		t.Fatalf("Unable to get spreadsheets: %v", err)
// 	}
// 	var sheetId int64
// 	sheetId = -1
// 	for _, s := range spreadsheet.Sheets {
// 		if s.Properties.Title == sheetTitleFixture(sheetNumber) {
// 			sheetId = s.Properties.SheetId
// 			break
// 		}
// 	}
// 	if sheetId == -1 {
// 		t.Fatalf(`Sheet ${sheetName} not found`)
// 	}
// 	data := dataFixture()
// 	gridCoordinate := gridCoordinateFixture(sheetNumber)
// 	pasteDataRequest := sheets.PasteDataRequest{
// 		Coordinate: gridCoordinate,
// 		Data:       data,
// 		Delimiter:  "|",
// 		Type:       "PASTE_NORMAL",
// 	}
// 	request := sheets.Request{
// 		PasteData: &pasteDataRequest,
// 	}

// 	batchUpdateSpreadsheetRequest := sheets.BatchUpdateSpreadsheetRequest{
// 		Requests: []*sheets.Request{
// 			&request,
// 		},
// 	}
// 	call := srv.Spreadsheets.BatchUpdate(
// 		spreadsheetId,
// 		&batchUpdateSpreadsheetRequest,
// 	)
// 	_, err = call.Context(context.Background()).Do()
// 	if err != nil {
// 		log.Fatalf("Unable to store data in sheet: %v", err)
// 	}
// }

func TestGoogleSheetsAPIMockServer(t *testing.T) {
	// GIVEN
	mockServer := mockServerFixture()
	client := mockServer.Client()
	sheetNumber := int64(1)
	spreadsheetId := spreadsheetIdFixture()

	// WHEN
	srv, err := sheets.New(client)
	// Update the basepath to use the mock url.
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
	data := dataFixture()
	gridCoordinate := gridCoordinateFixture(sheetNumber)
	pasteDataRequest := sheets.PasteDataRequest{
		Coordinate: gridCoordinate,
		Data:       data,
		Delimiter:  "|",
		Type:       "PASTE_NORMAL",
	}
	request := sheets.Request{
		PasteData: &pasteDataRequest,
	}

	batchUpdateSpreadsheetRequest := sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			&request,
		},
	}
	call := srv.Spreadsheets.BatchUpdate(
		spreadsheetId,
		&batchUpdateSpreadsheetRequest,
	)
	_, err = call.Context(context.Background()).Do()
	if err != nil {
		log.Fatalf("Unable to store data in sheet: %v", err)
	}
}
