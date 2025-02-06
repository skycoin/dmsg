package commands

var errorCode = map[string]int{
	"UNSUPPORTED_PROTOCOL":   1,
	"FAILED_INIT":            2,
	"URL_MALFORMAT":          3,
	"COULDNT_RESOLVE_PROXY":  5,
	"COULDNT_RESOLVE_HOST":   6,
	"COULDNT_CONNECT":        7,
	"WRITE_ERROR":            23,
	"READ_ERROR":             26,
	"OUT_OF_MEMORY":          27,
	"OPERATION_TIMEDOUT":     28,
	"FILE_COULDNT_READ_FILE": 37,
	"UNKNOWN_OPTION":         48,
	"GOT_NOTHING":            52,
	"SEND_ERROR":             55,
	"RECV_ERROR":             56,
	"FILESIZE_EXCEEDED":      63,
}

var errorDesc = map[string]string{
	"UNSUPPORTED_PROTOCOL":   "The URL you passed to dmsgcurl used a protocol that does not support.",
	"FAILED_INIT":            "Very early initialization code failed.",
	"URL_MALFORMAT":          "The URL was not properly formatted.",
	"COULDNT_RESOLVE_PROXY":  "Couldn't resolve proxy. The given proxy host could not be resolved.",
	"COULDNT_RESOLVE_HOST":   "Couldn't resolve host. The given remote host was not resolved.",
	"COULDNT_CONNECT":        "Failed to connect() to host or proxy.",
	"WRITE_ERROR":            "An error occurred when writing received data to a local file, or an error was returned to dmsgcurl from a write callback.",
	"READ_ERROR":             "There was a problem reading a local file or an error returned by the read callback.",
	"OUT_OF_MEMORY":          "A memory allocation request failed.",
	"OPERATION_TIMEDOUT":     "Operation timeout. The specified time-out period was reached according to the conditions.",
	"FILE_COULDNT_READ_FILE": "A file given with FILE:// couldn't be opened. Most likely because the file path doesn't identify an existing file.",
	"UNKNOWN_OPTION":         "An option passed to dmsgcurl is not recognized/known.",
	"GOT_NOTHING":            "Nothing was returned from the server, and under the circumstances, getting nothing is considered an error.",
	"SEND_ERROR":             "Failed sending network data.",
	"RECV_ERROR":             "Failure with receiving network data.",
	"FILESIZE_EXCEEDED":      "Maximum file size exceeded.",
}
