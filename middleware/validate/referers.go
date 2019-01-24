package validate

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/model/credential"
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/util"
)

func Referers() middleware.Middleware {
	return referers
}

func referers(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if reqCredential == credential.Permission {
			reqDomain := req.Header.Get("Referer")
			if reqDomain == "" {
				util.WriteBackError(w, "failed to identify request domain, empty header: Referer", http.StatusUnauthorized)
				return
			}

			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			allowedReferers := reqPermission.Referers

			var validated bool
			for _, referer := range allowedReferers {
				referer = strings.Replace(referer, "*", ".*", -1)
				matched, err := regexp.MatchString(referer, reqDomain)
				if err != nil {
					log.Printf("%s: %v\n", logTag, err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if matched {
					validated = true
					break
				}
			}

			if !validated {
				util.WriteBackError(w, "permission doeesn't have required referers", http.StatusInternalServerError)
				return
			}
		}

		h(w, req)
	}
}