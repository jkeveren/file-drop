package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Handler struct {
	html []byte
}

var html string = `
<body>
<script>
	const input = document.createElement("input");
	input.type = "file";
	input.multiple = true;
	document.body.appendChild(input);
	
	const sendButton = document.createElement("button");
	sendButton.innerText = "Send";
	document.body.appendChild(sendButton);

	sendButton.addEventListener("click", e => {
		for (let file of input.files) {
			
			const abortController = new AbortController();
			
			const fileElement = document.createElement("div");
			fileElement.innerText = file.name;
			document.body.appendChild(fileElement);

			const fileButton = document.createElement("button");
			fileButton.innerText = "Cancel"
			fileElement.appendChild(fileButton);
			fileButton.addEventListener("click", e => {
				fileElement.remove();
				abortController.abort();
			});
				
			(async () => {
				try {
					const query = new URLSearchParams({
						name: file.name,
						size: file.size,
						lastModified: file.lastModified
					});
					const response = await fetch(
						"?" + query.toString(),
						{
							method: "PUT",
							body: file,
							signal: abortController.signal
						}
					);
					if (response.ok) {
						fileButton.innerText = "Done";
					} else {
						fileButton.innerText = "Failed";
					}
				} catch (err) {
					console.error(err);
					fileButton.innerText = "Failed";
				}
				fileButton.disabled = true;
			})()
		}
	});
</script>
`

func main() {
	var port string
	args := os.Args[1:]
	if len(args) > 0 {
		port = args[0]
	} else {
		port = "8736"
	}
	fmt.Printf("Starting server on port \"%s\". Supply alternative port in first argument to change.\n", port)

	filler := make([]byte, 1e3)
	for i := range filler {
		filler[i] = 0x2d // UTF-8 hyphen
	}
	copy(filler, []byte("<!"))
	filler = append(filler, byte('>'), byte('\n'))
	handler := Handler{[]byte(html)}

	panic(http.ListenAndServe("0.0.0.0:"+port, handler))
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {

	case http.MethodGet:
		w.Header().Set("Content-Type", "text/html")
		w.Write(h.html)

	case http.MethodPut:
		values := r.URL.Query()
		fileName := values.Get("name")
		fmt.Println("Started:", fileName)

		tempFileName := "!IN-PROGRESS-" + fileName
		localFile, err := os.Create(tempFileName)
		if err != nil {
			serverError(w, err)
			return
		}
		defer localFile.Close()
		w.Header().Set("Content-Type", "text/html")
		b := make([]byte, 1e6)
		for {
			n, err := r.Body.Read(b)
			if err != nil && err != io.EOF {
				serverError(w, err)
				fmt.Println("Failed:", fileName)
				err = os.Remove(tempFileName)
				if err != nil {
					fmt.Println("Unable to remove", tempFileName)
				}
				return
			}
			if n == 0 {
				break
			}

			n, err = localFile.Write(b[:n])
			if err != nil {
				serverError(w, err)
				return
			}
		}
		tInt, err := strconv.Atoi(values.Get("lastModified"))
		if err != nil {
			fmt.Println(err)
			return
		}
		t := time.UnixMilli(int64(tInt))
		os.Chtimes(tempFileName, t, t)
		err = os.Rename(tempFileName, fileName)
		if err != nil {
			serverError(w, err)
			return
		}
		fmt.Println("Complete:", fileName)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func serverError(w http.ResponseWriter, err error) {
	fmt.Println(err)
	w.WriteHeader(http.StatusInternalServerError)
}
