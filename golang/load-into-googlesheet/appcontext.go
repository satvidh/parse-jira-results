package main

import (
	"errors"
	"os"

	"google.golang.org/api/sheets/v4"
	"local.dev/sheetsLoader/internal/config"
	"local.dev/sheetsLoader/internal/operator"
)

type appContext struct {
	CredentialsFilePath           string
	SpreadsheetId                 string
	CellRef                       string
	Delimiter                     string
	Scopes                        []string
	GoogleContextFactory          func() *config.GoogleContext
	InputContextFactory           func() *config.InputContext
	GoogleSheetsConnectionFactory func(googleCtx *config.GoogleContext, credentialsFilePath string, scope ...string) (operator.Connection, error)
	GoogleSheetsOutputFactory     func(spreadsheetId string, cellRef string, delimiter string) operator.Output
	ReaderConnectionFactory       func(inputContext *config.InputContext) operator.Connection
	DelimitedTextInputFactory     func() operator.Input
}

func newAppContext() (*appContext, error) {
	credentialsFilePath := os.Getenv("CREDENTIALS_FILEPATH")
	spreadsheetId := os.Getenv("SPREADSHEET_ID")
	cellRef := os.Getenv("CELL_REF")
	delimiter := os.Getenv("DELIMITER")
	if credentialsFilePath == "" || spreadsheetId == "" || cellRef == "" || delimiter == "" {
		return nil, errors.New("environment variables CREDENTIALS_FILEPATH, SPREADSHEET_ID, CELL_REF, and DELIMITER must be set")
	}
	err := error(nil)
	scopes := []string{
		sheets.SpreadsheetsScope,
	}

	return &appContext{
		CredentialsFilePath:           credentialsFilePath,
		SpreadsheetId:                 spreadsheetId,
		CellRef:                       cellRef,
		Delimiter:                     delimiter,
		Scopes:                        scopes,
		GoogleContextFactory:          config.NewGoogleContext,
		InputContextFactory:           config.NewInputContext,
		GoogleSheetsConnectionFactory: operator.NewGoogleSheetsConnection,
		GoogleSheetsOutputFactory:     operator.NewGoogleSheetsOutput,
		ReaderConnectionFactory:       operator.NewReaderConnection,
		DelimitedTextInputFactory:     operator.NewDelimitedTextInput,
	}, err
}
