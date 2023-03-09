package admin

import (
	"encoding/json"
	"net/http"

	"github.com/yinyajun/cron"
)

func renderJson(w http.ResponseWriter, resp interface{}) {
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
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

	h.mux.Handle("/add", newAddHandlerFunc(agent))
	h.mux.Handle("/active", newActiveHandlerFunc(agent))
	h.mux.Handle("/pause", newPauseHandlerFunc(agent))
	h.mux.Handle("/remove", newRemoveHandlerFunc(agent))
	h.mux.Handle("/execute", newExecuteOnceHandlerFunc(agent))
	h.mux.Handle("/running", newRunningHandlerFunc(agent))
	h.mux.Handle("/schedule", newScheduleHandlerFunc(agent))
	h.mux.Handle("/history", newHistoryHandlerFunc(agent))
	h.mux.Handle("/jobs", newJobsHandlerFunc(agent))

	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}
