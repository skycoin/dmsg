## DMSGCURL
#### Usage
```
$ skywire dmsg curl dmsg://{pk}:{port}/xxx
```

#### Errors
We trying to use same error's status code like what libcurl used as below:
| ERROR CODE | SHORT DESCRIPTION | LONG DESCRIPTION |
|---|---|---|
| 0 | OK | All fine. Proceed as usual. |
| 1 | UNSUPPORTED_PROTOCOL | The URL you passed to dmsgcurl used a protocol that does not support. |
| 2 | FAILED_INIT | Very early initialization code failed. |
| 3 | URL_MALFORMAT | The URL was not properly formatted. |
| 5 | COULDNT_RESOLVE_PROXY | Couldn't resolve proxy. The given proxy host could not be resolved. |
| 6 | COULDNT_RESOLVE_HOST | Couldn't resolve host. The given remote host was not resolved. |
| 7 | COULDNT_CONNECT | Failed to connect() to host or proxy. |
| 23 | WRITE_ERROR | An error occurred when writing received data to a local file, or an error was returned to dmsgcurl from a write callback. |
| 26 | READ_ERROR | There was a problem reading a local file or an error returned by the read callback. |
| 27 | OUT_OF_MEMORY | A memory allocation request failed. |
| 28 | OPERATION_TIMEDOUT | Operation timeout. The specified time-out period was reached according to the conditions. |
| 37 | FILE_COULDNT_READ_FILE | A file given with FILE:// couldn't be opened. Most likely because the file path doesn't identify an existing file. |
| 48 | UNKNOWN_OPTION | An option passed to dmsgcurl is not recognized/known. |
| 52 | GOT_NOTHING | Nothing was returned from the server, and under the circumstances, getting nothing is considered an error. |
| 55 | SEND_ERROR | Failed sending network data. |
| 56 | RECV_ERROR | Failure with receiving network data. |
| 63 | FILESIZE_EXCEEDED | Maximum file size exceeded. |

