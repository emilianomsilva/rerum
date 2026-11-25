package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"rerum/backend/pkg/application"
)

func (s *Server) analyticsSeries(w http.ResponseWriter, r *http.Request) {
	filter, err := parseAnalyticsFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if err := application.ValidateAnalyticsFilter(filter); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	result, err := (application.AnalyticsService{DB: s.Store.DB}).Series(r.Context(), filter)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func parseAnalyticsFilter(r *http.Request) (application.AnalyticsFilter, error) {
	query := r.URL.Query()
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -11, 0)
	end := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	filter := application.AnalyticsFilter{
		Inicio:        start,
		Fim:           end,
		Eixo:          "tempo",
		Series:        "tipo",
		Metrica:       "ambos",
		Granularidade: "mes",
	}
	if value := strings.TrimSpace(query.Get("inicio")); value != "" {
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			return filter, err
		}
		filter.Inicio = parsed
	}
	if value := strings.TrimSpace(query.Get("fim")); value != "" {
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			return filter, err
		}
		filter.Fim = parsed
	}
	if value := strings.TrimSpace(query.Get("eixo")); value != "" {
		filter.Eixo = value
	}
	if value := strings.TrimSpace(query.Get("series")); value != "" {
		filter.Series = value
	}
	if value := strings.TrimSpace(query.Get("metrica")); value != "" {
		filter.Metrica = value
	}
	if value := strings.TrimSpace(query.Get("granularidade")); value != "" {
		filter.Granularidade = value
	}
	if value := strings.TrimSpace(query.Get("imoveis")); value != "" {
		seen := map[int64]bool{}
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil || id < 1 {
				return filter, errInvalidAnalyticsProperty
			}
			if !seen[id] {
				seen[id] = true
				filter.ImovelIDs = append(filter.ImovelIDs, id)
			}
		}
	}
	return filter, nil
}

type analyticsError string

func (e analyticsError) Error() string { return string(e) }

const errInvalidAnalyticsProperty analyticsError = "filtro de imóveis inválido"
