package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/KosukeOhmura/room_crawler/src"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := src.Execute(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			if notifyErr := src.NotifyError(err); notifyErr != nil {
				errs := fmt.Errorf("failed to notify err. notify err: %s, err: %s", notifyErr, err)
				fmt.Printf(errs.Error())
				w.Write([]byte(errs.Error()))
			} else {
				w.Write([]byte(err.Error()))
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	http.ListenAndServe(fmt.Sprintf(":%s", os.Getenv("PORT")), nil)
}
