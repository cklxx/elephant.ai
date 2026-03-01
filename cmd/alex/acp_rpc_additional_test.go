package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRPCPayloadRequest(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"client":"alex"}}`)

	req, resp, err := parseRPCPayload(payload)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Nil(t, resp)
	require.Equal(t, "initialize", req.Method)
	require.Equal(t, float64(1), req.ID)
	require.Equal(t, "alex", req.Params["client"])
}

func TestParseRPCPayloadResponse(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":"7","result":{"ok":true}}`)

	req, resp, err := parseRPCPayload(payload)
	require.NoError(t, err)
	require.Nil(t, req)
	require.NotNil(t, resp)
	require.Equal(t, "7", resp.ID)

	resultMap, ok := resp.Result.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, resultMap["ok"])
}

func TestParseRPCPayloadInvalidJSON(t *testing.T) {
	req, resp, err := parseRPCPayload([]byte(`{"jsonrpc":"2.0",`))
	require.Error(t, err)
	require.Nil(t, req)
	require.Nil(t, resp)
}

func TestBytesTrimSpace(t *testing.T) {
	input := []byte("  \n\t hello rpc \r\n ")
	trimmed := bytesTrimSpace(input)
	require.Equal(t, []byte("hello rpc"), trimmed)
}

func TestParseRPCPayloadResponseErrorObject(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":9,"error":{"code":-32601,"message":"not found"}}`)

	req, resp, err := parseRPCPayload(payload)
	require.NoError(t, err)
	require.Nil(t, req)
	require.NotNil(t, resp)
	require.True(t, resp.IsError())
	require.NotNil(t, resp.Error)
	require.Equal(t, -32601, resp.Error.Code)
	require.Equal(t, "not found", resp.Error.Message)
}

