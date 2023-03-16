package admin

import (
	"encoding/json"
	"fmt"
	cron_admin "github.com/yinyajun/cron-admin"
	"net/http"

	"github.com/yinyajun/cron"
)

func renderJson(w http.ResponseWriter, resp interface{}) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func newAddHandlerFunc(agent *cron.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		spec := query.Get("spec")
		job := query.Get("job")
		if err := agent.Add(spec, job); err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		renderJson(w, "ok")
	}
}

func newActiveHandlerFunc(agent *cron.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		job := query.Get("job")
		if err := agent.Active(job); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Println(">>", "active", job)
		renderJson(w, "ok")
	}
}

func newPauseHandlerFunc(agent *cron.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		job := query.Get("job")
		if err := agent.Pause(job); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Println(">>", "pause", job)
		renderJson(w, "ok")
	}
}

func newRemoveHandlerFunc(agent *cron.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		job := query.Get("job")
		if err := agent.Remove(job); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJson(w, "ok")
	}
}

func newExecuteOnceHandlerFunc(agent *cron.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		job := query.Get("job")
		if err := agent.ExecuteOnce(job); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJson(w, "ok")
	}
}

func newScheduleHandlerFunc(agent *cron.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		events, err := agent.Schedule()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJson(w, events)
	}
}

func newRunningHandlerFunc(agent *cron.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		executions, err := agent.Running()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJson(w, executions)
	}
}

func newHistoryHandlerFunc(agent *cron.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		job := query.Get("job")
		executions, err := agent.History(job)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		renderJson(w, executions)
	}
}

func newJobsHandlerFunc(agent *cron.Agent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobs := agent.Jobs()
		renderJson(w, jobs)
	}
}

type Handler struct {
	mux *http.ServeMux
}

func NewHandler(agent *cron.Agent) *Handler {
	h := &Handler{mux: http.NewServeMux()}

	h.mux.Handle("/api/v1/add", newAddHandlerFunc(agent))
	h.mux.Handle("/api/v1/active", newActiveHandlerFunc(agent))
	h.mux.Handle("/api/v1/pause", newPauseHandlerFunc(agent))
	h.mux.Handle("/api/v1/remove", newRemoveHandlerFunc(agent))
	h.mux.Handle("/api/v1/execute", newExecuteOnceHandlerFunc(agent))
	h.mux.Handle("/api/v1/running", newRunningHandlerFunc(agent))
	h.mux.Handle("/api/v1/schedule", newScheduleHandlerFunc(agent))
	h.mux.Handle("/api/v1/history", newHistoryHandlerFunc(agent))
	h.mux.Handle("/api/v1/jobs", newJobsHandlerFunc(agent))

	h.mux.Handle("/", cron_admin.UIHandler())

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}
