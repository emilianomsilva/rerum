package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rerum/backend/pkg/application"
	"rerum/backend/pkg/domain"
	"rerum/backend/pkg/importer"
	"rerum/backend/pkg/storage/sqlite"
)

type Server struct{ Store *sqlite.Store }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		jsonResponse(w, 200, map[string]string{"status": "ok", "backend": "go"})
	})
	mux.HandleFunc("GET /api/imoveis", s.listProperties)
	mux.HandleFunc("POST /api/imoveis", s.createProperty)
	mux.HandleFunc("POST /api/imoveis/lote", s.createPropertyBatch)
	mux.HandleFunc("POST /api/imoveis/lote/previa", s.previewPropertyBatch)
	mux.HandleFunc("GET /api/imoveis/{id}", s.getProperty)
	mux.HandleFunc("PATCH /api/imoveis/{id}", s.updateProperty)
	mux.HandleFunc("POST /api/imoveis/{id}/reativar", s.reactivateProperty)
	mux.HandleFunc("DELETE /api/imoveis/{id}/cascade", s.deletePropertyCascade)
	mux.HandleFunc("DELETE /api/imoveis/{id}", s.deactivateProperty)
	mux.HandleFunc("GET /api/inquilinos", s.listTenants)
	mux.HandleFunc("POST /api/inquilinos", s.createTenant)
	mux.HandleFunc("GET /api/inquilinos/{id}", s.getTenant)
	mux.HandleFunc("PATCH /api/inquilinos/{id}", s.updateTenant)
	mux.HandleFunc("POST /api/inquilinos/{id}/reativar", s.reactivateTenant)
	mux.HandleFunc("DELETE /api/inquilinos/{id}/cascade", s.deleteTenantCascade)
	mux.HandleFunc("DELETE /api/inquilinos/{id}", s.deactivateTenant)
	mux.HandleFunc("GET /api/locacoes", s.listLeases)
	mux.HandleFunc("GET /api/locacoes/{id}", s.getLease)
	mux.HandleFunc("POST /api/locacoes", s.createLease)
	mux.HandleFunc("PATCH /api/locacoes/{id}", s.updateLease)
	mux.HandleFunc("POST /api/locacoes/{id}/renovar", s.renewLease)
	mux.HandleFunc("DELETE /api/locacoes/{id}", s.deleteLease)
	mux.HandleFunc("POST /api/locacoes/{id}/encerrar", s.closeLease)
	mux.HandleFunc("GET /api/parcelas", s.listInstallments)
	mux.HandleFunc("POST /api/parcelas", s.createExtraInstallment)
	mux.HandleFunc("GET /api/parcelas/{id}", s.getInstallment)
	mux.HandleFunc("PATCH /api/parcelas/{id}", s.updateInstallment)
	mux.HandleFunc("DELETE /api/parcelas/{id}", s.deleteInstallment)
	mux.HandleFunc("POST /api/parcelas/{id}/cancelar", s.cancelInstallment)
	mux.HandleFunc("POST /api/parcelas/{id}/quitar", s.payInstallment)
	mux.HandleFunc("POST /api/parcelas/{id}/estornar", s.reopenInstallment)
	mux.HandleFunc("GET /api/parcelas/{id}/simular", s.simulateInstallment)
	mux.HandleFunc("GET /recibos/{file}", s.downloadReceipt)
	mux.HandleFunc("POST /api/parcelas/quitar-lote", s.payInstallmentBatch)
	mux.HandleFunc("POST /api/locacoes/{id}/cancelar-futuras", s.cancelFutureInstallments)
	mux.HandleFunc("GET /api/despesas", s.listExpenses)
	mux.HandleFunc("POST /api/despesas", s.createExpense)
	mux.HandleFunc("DELETE /api/despesas/{id}", s.deleteExpense)
	mux.HandleFunc("GET /api/empreendimentos", s.listProjects)
	mux.HandleFunc("POST /api/empreendimentos", s.createProject)
	mux.HandleFunc("GET /api/empreendimentos/{id}", s.getProject)
	mux.HandleFunc("PATCH /api/empreendimentos/{id}", s.updateProject)
	mux.HandleFunc("DELETE /api/empreendimentos/{id}", s.deleteProject)
	mux.HandleFunc("POST /api/empreendimentos/{id}/reativar", s.reactivateProject)
	mux.HandleFunc("DELETE /api/empreendimentos/{id}/permanente", s.deleteUnusedProject)
	mux.HandleFunc("GET /api/movimentacoes", s.listMovements)
	mux.HandleFunc("POST /api/movimentacoes", s.createMovement)
	mux.HandleFunc("PATCH /api/movimentacoes/{id}", s.updateMovement)
	mux.HandleFunc("DELETE /api/movimentacoes/{id}", s.deleteMovement)
	mux.HandleFunc("GET /api/movimentacoes/{id}/rateios", s.listAllocations)
	mux.HandleFunc("GET /api/dividas", s.listDebts)
	mux.HandleFunc("POST /api/dividas", s.createDebt)
	mux.HandleFunc("PATCH /api/dividas/{id}", s.updateDebt)
	mux.HandleFunc("DELETE /api/dividas/{id}", s.deleteDebt)
	mux.HandleFunc("POST /api/dividas/{id}/movimentos", s.addDebtMovement)
	mux.HandleFunc("DELETE /api/dividas/movimentos/{id}", s.deleteDebtMovement)
	mux.HandleFunc("GET /api/dashboard", s.dashboard)
	mux.HandleFunc("GET /api/imoveis/{id}/roi", s.propertyROI)
	mux.HandleFunc("GET /api/estatisticas", s.statistics)
	mux.HandleFunc("GET /api/relatorios/anual", s.annualReport)
	mux.HandleFunc("GET /api/analytics/series", s.analyticsSeries)
	mux.HandleFunc("GET /api/backup/sqlite", s.backupSQLite)
	mux.HandleFunc("GET /api/backup/full", s.backupFull)
	mux.HandleFunc("POST /api/import/preview", s.previewWorkbook)
	mux.HandleFunc("POST /api/import/execute", s.importWorkbook)
	return mux
}

func (s *Server) listProperties(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.Properties(r.Context(), r.URL.Query().Get("incluir_inativos") == "true")
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (s *Server) getProperty(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	item, err := s.Store.Property(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (s *Server) createProperty(w http.ResponseWriter, r *http.Request) {
	var item domain.Imovel
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	if item.Status != "disponivel" && item.Status != "manutencao" {
		http.Error(w, "status inválido", http.StatusUnprocessableEntity)
		return
	}
	item.Ativo = true
	saved, err := s.Store.SaveProperty(r.Context(), item)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, saved)
}

func (s *Server) updateProperty(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	var item domain.Imovel
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	if item.Status != "disponivel" && item.Status != "manutencao" {
		http.Error(w, "status inválido; ocupação é determinada pela locação ativa", http.StatusUnprocessableEntity)
		return
	}
	item.ID = id
	current, err := s.Store.Property(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	item.Ativo = current.Ativo
	item.InvestimentoImportadoC = current.InvestimentoImportadoC
	saved, err := s.Store.SaveProperty(r.Context(), item)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, saved)
}

func (s *Server) deactivateProperty(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.Store.DeactivateProperty(r.Context(), id); err != nil {
		apiError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reactivateProperty(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.Store.ReactivateProperty(r.Context(), id); err != nil {
		apiError(w, err)
		return
	}
	item, err := s.Store.Property(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (s *Server) deletePropertyCascade(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	backup, err := s.Store.DeletePropertyCascade(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"backup_path": backup})
}

func (s *Server) listTenants(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.Tenants(r.Context(), r.URL.Query().Get("incluir_inativos") == "true")
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (s *Server) getTenant(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	item, err := s.Store.Tenant(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (s *Server) createTenant(w http.ResponseWriter, r *http.Request) {
	var item domain.Inquilino
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	item.Ativo = true
	saved, err := s.Store.SaveTenant(r.Context(), item)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, saved)
}

func (s *Server) updateTenant(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	var item domain.Inquilino
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	current, err := s.Store.Tenant(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	item.ID, item.Ativo = id, current.Ativo
	saved, err := s.Store.SaveTenant(r.Context(), item)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, saved)
}

func (s *Server) deactivateTenant(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.Store.DeactivateTenant(r.Context(), id); err != nil {
		apiError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) reactivateTenant(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.Store.ReactivateTenant(r.Context(), id); err != nil {
		apiError(w, err)
		return
	}
	item, err := s.Store.Tenant(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (s *Server) deleteTenantCascade(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	backup, err := s.Store.DeleteTenantCascade(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"backup_path": backup})
}

func (s *Server) listLeases(w http.ResponseWriter, r *http.Request) {
	var propertyID *int64
	if value := r.URL.Query().Get("imovel_id"); value != "" {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			apiError(w, errors.New("imovel_id inválido"))
			return
		}
		propertyID = &id
	}
	var active *bool
	if value := r.URL.Query().Get("ativas"); value != "" {
		parsed := value == "true" || value == "1"
		active = &parsed
	}
	items, err := s.Store.LeaseSummaries(r.Context(), propertyID, active)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (s *Server) getLease(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	item, err := s.Store.Lease(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (s *Server) createLease(w http.ResponseWriter, r *http.Request) {
	var item domain.Locacao
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	saved, installments, err := s.Store.CreateLease(r.Context(), item)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]any{"locacao": saved, "parcelas": installments, "parcelas_geradas": len(installments)})
}

func (s *Server) updateLease(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	var item domain.Locacao
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	item.ID = id
	updated, err := s.Store.UpdateLease(r.Context(), item)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, updated)
}

func (s *Server) renewLease(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	var input struct {
		NovoFim         string `json:"novo_fim"`
		NovoValorMensal int64  `json:"novo_valor_mensal_c"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apiError(w, err)
		return
	}
	lease, installments, err := s.Store.RenewLease(r.Context(), id, input.NovoFim, input.NovoValorMensal)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"locacao": lease, "parcelas": installments, "novas_parcelas": len(installments)})
}

func (s *Server) deleteLease(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.Store.DeleteLease(r.Context(), id); err != nil {
		apiError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) closeLease(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	count, err := s.Store.CloseLease(r.Context(), id, time.Now().Format("2006-01-02"))
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]int{"canceladas": count})
}

func (s *Server) cancelFutureInstallments(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	today, err := requestDate(r)
	if err != nil {
		apiError(w, err)
		return
	}
	result, err := s.Store.CancelFutureInstallments(r.Context(), id, today.Format("2006-01-02"))
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]int{"canceladas": result})
}

func (s *Server) listInstallments(w http.ResponseWriter, r *http.Request) {
	var leaseID, propertyID *int64
	for key, target := range map[string]**int64{"locacao_id": &leaseID, "imovel_id": &propertyID} {
		if value := r.URL.Query().Get(key); value != "" {
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				apiError(w, errors.New("filtro inválido"))
				return
			}
			*target = &id
		}
	}
	today, err := requestDate(r)
	if err != nil {
		apiError(w, err)
		return
	}
	items, err := s.Store.Installments(r.Context(), leaseID, propertyID)
	if err != nil {
		apiError(w, err)
		return
	}
	for index := range items {
		items[index].Situacao = sqlite.Situation(items[index], today)
		if _, ok := s.resolvedReceiptPath(items[index]); ok {
			items[index].TemRecibo = true
		}
	}
	if situation := r.URL.Query().Get("situacao"); situation != "" {
		filtered := items[:0]
		for _, item := range items {
			if item.Situacao == situation || (situation == "a_receber" && (item.Situacao == "aberta" || item.Situacao == "vencida" || item.Situacao == "parcial" || item.Situacao == "parcial-vencida")) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	jsonResponse(w, http.StatusOK, items)
}

func (s *Server) getInstallment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	item, err := s.Store.Installment(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	if _, ok := s.resolvedReceiptPath(item); ok {
		item.TemRecibo = true
	}
	jsonResponse(w, http.StatusOK, item)
}

func (s *Server) createExtraInstallment(w http.ResponseWriter, r *http.Request) {
	var item domain.Parcela
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	saved, err := s.Store.CreateExtraInstallment(r.Context(), item)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, saved)
}

func (s *Server) updateInstallment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	var input sqlite.InstallmentUpdate
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apiError(w, err)
		return
	}
	item, err := s.Store.UpdateOpenInstallment(r.Context(), id, input)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (s *Server) reopenInstallment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	item, err := s.Store.ReopenInstallment(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (s *Server) payInstallment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	var input sqlite.InstallmentPayment
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apiError(w, err)
		return
	}
	if input.Data == "" {
		input.Data = time.Now().Format("2006-01-02")
	}
	if input.Forma == "" {
		input.Forma = "pix"
	}
	input.ParcelaID = id
	items, err := s.Store.PayInstallments(r.Context(), []sqlite.InstallmentPayment{input})
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, items[0])
}

func (s *Server) simulateInstallment(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	today, err := requestDate(r)
	if err != nil {
		apiError(w, err)
		return
	}
	result, err := s.Store.SimulateInstallment(r.Context(), id, today.Format("2006-01-02"))
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (s *Server) downloadReceipt(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSuffix(r.PathValue("file"), ".pdf"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	item, err := s.Store.Installment(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	path, ok := s.resolvedReceiptPath(item)
	if !ok {
		http.Error(w, "recibo não encontrado", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=recibo_%d.pdf", id))
	http.ServeFile(w, r, path)
}

// resolvedReceiptPath prefers the current receipts directory and also accepts
// historic absolute paths for receipts stored elsewhere.
func (s *Server) resolvedReceiptPath(item domain.Parcela) (string, bool) {
	if item.ReciboPath == nil || strings.TrimSpace(*item.ReciboPath) == "" {
		return "", false
	}
	if strings.TrimSpace(s.Store.ReceiptsDir) != "" {
		candidate := filepath.Join(s.Store.ReceiptsDir, filepath.Base(*item.ReciboPath))
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	if info, err := os.Stat(*item.ReciboPath); err == nil && !info.IsDir() {
		return *item.ReciboPath, true
	}
	return "", false
}

func (s *Server) payInstallmentBatch(w http.ResponseWriter, r *http.Request) {
	var input struct {
		IDs        []int64                     `json:"ids"`
		Data       string                      `json:"data"`
		Forma      string                      `json:"forma"`
		Pagamentos []sqlite.InstallmentPayment `json:"pagamentos"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apiError(w, err)
		return
	}
	if input.Pagamentos != nil && input.IDs != nil {
		http.Error(w, "informe pagamentos individuais ou IDs, não ambos", http.StatusUnprocessableEntity)
		return
	}
	if input.Pagamentos == nil {
		if input.Data == "" {
			input.Data = time.Now().Format("2006-01-02")
		}
		for _, id := range input.IDs {
			input.Pagamentos = append(input.Pagamentos, sqlite.InstallmentPayment{ParcelaID: id, Data: input.Data, Forma: input.Forma})
		}
	} else {
		for _, payment := range input.Pagamentos {
			if payment.Data == "" || payment.ValorPagoC == nil {
				http.Error(w, "informe data e valor recebido para cada parcela", http.StatusUnprocessableEntity)
				return
			}
		}
	}
	items, err := s.Store.PayInstallments(r.Context(), input.Pagamentos)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (s *Server) listExpenses(w http.ResponseWriter, r *http.Request) {
	var propertyID *int64
	if value := r.URL.Query().Get("imovel_id"); value != "" {
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			apiError(w, err)
			return
		}
		propertyID = &id
	}
	items, err := s.Store.Expenses(r.Context(), propertyID)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, items)
}
func (s *Server) createExpense(w http.ResponseWriter, r *http.Request) {
	var item domain.Despesa
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	saved, err := s.Store.SaveExpense(r.Context(), item)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, saved)
}
func (s *Server) deleteExpense(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.Store.DeleteExpense(r.Context(), id); err != nil {
		apiError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.Projects(r.Context(), r.URL.Query().Get("incluir_inativos") == "true")
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	item, err := s.Store.Project(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var item domain.Empreendimento
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	saved, err := s.Store.SaveProject(r.Context(), item)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, saved)
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	var item domain.Empreendimento
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	item.ID = id
	if err := s.Store.UpdateProject(r.Context(), item); err != nil {
		apiError(w, err)
		return
	}
	updated, err := s.Store.Project(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, updated)
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.Store.DeleteProject(r.Context(), id); err != nil {
		apiError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listMovements(w http.ResponseWriter, r *http.Request) {
	var f sqlite.MovementFilter
	f.Kind = r.URL.Query().Get("tipo")
	f.Natureza = r.URL.Query().Get("natureza")
	f.Start = r.URL.Query().Get("inicio")
	f.End = r.URL.Query().Get("fim")
	if value := r.URL.Query().Get("imovel_id"); value != "" {
		if id, err := strconv.ParseInt(value, 10, 64); err == nil {
			f.PropertyID = &id
		}
	}
	if value := r.URL.Query().Get("empreendimento_id"); value != "" {
		if id, err := strconv.ParseInt(value, 10, 64); err == nil {
			f.EmpreendimentoID = &id
		}
	}
	items, err := s.Store.FilterMovements(r.Context(), f)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (s *Server) createMovement(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Movimento domain.Movimento `json:"movimento"`
		Rateios   []domain.Rateio  `json:"rateios"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apiError(w, err)
		return
	}
	saved, err := s.Store.CreateMovement(r.Context(), input.Movimento, input.Rateios)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, saved)
}

func (s *Server) updateMovement(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	var input struct {
		Movimento domain.Movimento `json:"movimento"`
		Rateios   []domain.Rateio  `json:"rateios"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apiError(w, err)
		return
	}
	input.Movimento.ID = id
	if err := s.Store.UpdateMovement(r.Context(), input.Movimento, input.Rateios); err != nil {
		apiError(w, err)
		return
	}
	updated, err := s.Store.Movement(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, updated)
}

func (s *Server) deleteMovement(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.Store.DeleteMovement(r.Context(), id); err != nil {
		apiError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listAllocations(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	items, err := s.Store.Allocations(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (s *Server) listDebts(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.Debts(r.Context())
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, items)
}

func (s *Server) createDebt(w http.ResponseWriter, r *http.Request) {
	var item domain.Divida
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	saved, err := s.Store.CreateDebt(r.Context(), item)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, saved)
}

func (s *Server) updateDebt(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	var item domain.Divida
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	item.ID = id
	if err := s.Store.UpdateDebt(r.Context(), item); err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, item)
}

func (s *Server) deleteDebt(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.Store.DeleteDebt(r.Context(), id); err != nil {
		apiError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) addDebtMovement(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	var item domain.MovimentoDivida
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		apiError(w, err)
		return
	}
	item.DividaID = id
	if err := s.Store.AddDebtMovement(r.Context(), item); err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusCreated, item)
}

func (s *Server) deleteDebtMovement(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if err := s.Store.DeleteDebtMovement(r.Context(), id); err != nil {
		apiError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	today, err := requestDate(r)
	if err != nil {
		apiError(w, err)
		return
	}
	start, end, hasRange, err := requestRange(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if !hasRange {
		start, end = today.AddDate(0, 0, -365), today
	}
	result, err := (application.ROIService{DB: s.Store.DB}).DashboardRange(r.Context(), today, start, end)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}
func (s *Server) propertyROI(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		apiError(w, err)
		return
	}
	today, err := requestDate(r)
	if err != nil {
		apiError(w, err)
		return
	}
	start, end, hasRange, err := requestRange(r)
	if err != nil {
		apiError(w, err)
		return
	}
	if !hasRange {
		start, end = today.AddDate(0, 0, -365), today
	}
	result, err := (application.ROIService{DB: s.Store.DB}).PropertyRange(r.Context(), id, start, end)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "imóvel não encontrado", http.StatusNotFound)
		return
	}
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (s *Server) statistics(w http.ResponseWriter, r *http.Request) {
	unitsOnly := r.URL.Query().Get("modo") != "todos"
	start, end, _, err := requestRange(r)
	if err != nil {
		apiError(w, err)
		return
	}
	result, err := (application.StatisticsService{DB: s.Store.DB}).PortfolioRange(r.Context(), unitsOnly, start, end)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}

func (s *Server) annualReport(w http.ResponseWriter, r *http.Request) {
	yearStr := r.URL.Query().Get("ano")
	year := time.Now().Year()
	if yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil && y > 2000 {
			year = y
		}
	}
	result, err := (application.StatisticsService{DB: s.Store.DB}).AnnualReport(r.Context(), year)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, result)
}
func requestDate(r *http.Request) (time.Time, error) {
	if value := r.URL.Query().Get("h"); value != "" {
		return time.Parse("2006-01-02", value)
	}
	return time.Now(), nil
}

func requestRange(r *http.Request) (time.Time, time.Time, bool, error) {
	startRaw := strings.TrimSpace(r.URL.Query().Get("inicio"))
	endRaw := strings.TrimSpace(r.URL.Query().Get("fim"))
	if startRaw == "" || endRaw == "" {
		return time.Time{}, time.Time{}, false, nil
	}
	start, err := time.Parse("2006-01-02", startRaw)
	if err != nil {
		return time.Time{}, time.Time{}, false, errors.New("data inicial inválida")
	}
	end, err := time.Parse("2006-01-02", endRaw)
	if err != nil {
		return time.Time{}, time.Time{}, false, errors.New("data final inválida")
	}
	return start, end, true, nil
}

func (s *Server) backupSQLite(w http.ResponseWriter, r *http.Request) {
	file, err := os.CreateTemp("", "rerum-backup-*.sqlite")
	if err != nil {
		apiError(w, err)
		return
	}
	path := file.Name()
	_ = file.Close()
	_ = os.Remove(path)
	defer os.Remove(path)
	if _, err := s.Store.DB.ExecContext(r.Context(), `VACUUM INTO ?`, filepath.Clean(path)); err != nil {
		apiError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/x-sqlite3")
	w.Header().Set("Content-Disposition", "attachment; filename=rerum.sqlite")
	http.ServeFile(w, r, path)
}

func (s *Server) previewWorkbook(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apiError(w, err)
		return
	}
	preview, err := (importer.Workbook{DB: s.Store.DB}).Preview(input.Path)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, preview)
}
func (s *Server) importWorkbook(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		apiError(w, err)
		return
	}
	preview, backup, err := (importer.Workbook{DB: s.Store.DB}).Import(r.Context(), input.Path)
	if err != nil {
		apiError(w, err)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"preview": preview, "backup": backup})
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("identificador inválido")
	}
	return id, nil
}

func apiError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, sqlite.ErrNotFound) {
		status = http.StatusNotFound
	}
	message := strings.ToLower(err.Error())
	if errors.Is(err, sqlite.ErrProjectInUse) || strings.Contains(message, "unique constraint failed: empreendimentos.nome") || strings.Contains(message, "já possui locação") || strings.Contains(message, "já possui locacao") {
		status = http.StatusConflict
	} else if strings.Contains(message, "obrigat") || strings.Contains(message, "inválid") || strings.Contains(message, "inval") || strings.Contains(message, "deve ser") || strings.Contains(message, "deve conter") || strings.Contains(message, "duplicada no lote") || strings.Contains(message, "parcela bloqueada") || strings.Contains(message, "não pode") || strings.Contains(message, "nao pode") || strings.Contains(message, "não gerou") || strings.Contains(message, "nao gerou") || strings.Contains(message, "foreign key constraint") {
		status = http.StatusUnprocessableEntity
	}
	http.Error(w, err.Error(), status)
}

func jsonResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
