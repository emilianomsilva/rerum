package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"rerum/backend/pkg/domain"
	"rerum/backend/pkg/storage/sqlite"
)

func portfolioRequest(t *testing.T, handler http.Handler, method, path string, input any, status int) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if input != nil {
		if err := json.NewEncoder(&body).Encode(input); err != nil {
			t.Fatal(err)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, path, &body))
	if response.Code != status {
		t.Fatalf("%s %s: got %d, want %d: %s", method, path, response.Code, status, response.Body.String())
	}
	return response
}

func portfolioProperty(name string) domain.Imovel {
	return domain.Imovel{Apelido: name, Tipo: "casa", Status: "disponivel", DestinadoLocacao: true}
}

func TestPropertyBatchMixedNumbersRegistriesAndExactCosts(t *testing.T) {
	db := testDB(t)
	handler := (&Server{Store: &sqlite.Store{DB: db}}).Handler()
	portfolioRequest(t, handler, "POST", "/api/empreendimentos", map[string]any{"nome": "Portugal"}, 201)
	projectID := int64(1)
	batch := sqlite.PropertyBatch{ModoCusto: "dividir", CompraC: 101, ReformaC: 7, OutrosC: 5}
	for i := 0; i < 4; i++ {
		p := portfolioProperty(fmt.Sprintf("Portugal %d-%d", 620+(i/2)*10, i%2+1))
		p.Numero = fmt.Sprint(620 + (i/2)*10)
		p.Unidade = fmt.Sprint(i%2 + 1)
		p.EmpreendimentoID = &projectID
		p.Endereco = "Rua Portugal, " + p.Numero + " - Unidade " + p.Unidade
		p.MatriculaCartorio = "10.001"
		p.MatriculaCompartilhada = i < 2
		if i >= 2 {
			p.MatriculaCartorio = fmt.Sprint(20000 + i)
		}
		batch.Imoveis = append(batch.Imoveis, p)
	}
	preview := portfolioRequest(t, handler, "POST", "/api/imoveis/lote/previa", batch, 200)
	var expected []domain.Imovel
	if err := json.Unmarshal(preview.Body.Bytes(), &expected); err != nil {
		t.Fatal(err)
	}
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM imoveis`).Scan(&count)
	if count != 0 {
		t.Fatal("preview wrote properties")
	}
	created := portfolioRequest(t, handler, "POST", "/api/imoveis/lote", batch, 201)
	var saved []domain.Imovel
	if err := json.Unmarshal(created.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	var purchase, reform, other int64
	for i, property := range saved {
		purchase += property.ValorCompraC
		reform += property.ReformaC
		other += property.OutrosCustosC
		if property.ID == 0 || property.Numero != batch.Imoveis[i].Numero || property.Unidade != batch.Imoveis[i].Unidade || property.MatriculaCartorio != batch.Imoveis[i].MatriculaCartorio || property.MatriculaCompartilhada != (i < 2) {
			t.Fatalf("unit identification lost: %+v", property)
		}
		if property.ValorCompraC != expected[i].ValorCompraC || property.InvestimentoTotalC != expected[i].InvestimentoTotalC {
			t.Fatal("save differs from authoritative cost preview")
		}
		response := portfolioRequest(t, handler, "GET", fmt.Sprintf("/api/imoveis/%d", property.ID), nil, 200)
		var fetched domain.Imovel
		_ = json.Unmarshal(response.Body.Bytes(), &fetched)
		if fetched.Numero != property.Numero || fetched.MatriculaCartorio != property.MatriculaCartorio {
			t.Fatal("identification not persisted")
		}
	}
	if len(saved) != 4 || purchase != 101 || reform != 7 || other != 5 || saved[0].ValorCompraC != 26 || saved[3].ValorCompraC != 25 {
		t.Fatalf("cost totals not conserved: %d/%d/%d %+v", purchase, reform, other, saved)
	}
	// Editing an individual property must retain the explicit number/unit and registry.
	saved[3].Unidade = "B"
	saved[3].MatriculaCartorio = "30.001"
	portfolioRequest(t, handler, "PATCH", fmt.Sprintf("/api/imoveis/%d", saved[3].ID), saved[3], 200)
	var unit, registry string
	_ = db.QueryRow(`SELECT unidade,matricula_cartorio FROM imoveis WHERE id=?`, saved[3].ID).Scan(&unit, &registry)
	if unit != "B" || registry != "30.001" {
		t.Fatal("individual update discarded identification")
	}
}

func TestPropertyBatchRollbackAndValidation(t *testing.T) {
	for _, scenario := range []string{"foreign project", "archived project", "duplicate", "invalid status", "empty", "oversize", "negative cost", "invalid mode", "existing ID"} {
		t.Run(scenario, func(t *testing.T) {
			db := testDB(t)
			handler := (&Server{Store: &sqlite.Store{DB: db}}).Handler()
			batch := sqlite.PropertyBatch{ModoCusto: "dividir", CompraC: 101, Imoveis: []domain.Imovel{portfolioProperty("Primeira"), portfolioProperty("Segunda")}}
			switch scenario {
			case "foreign project":
				id := int64(999)
				batch.Imoveis[1].EmpreendimentoID = &id
			case "archived project":
				portfolioRequest(t, handler, "POST", "/api/empreendimentos", map[string]string{"nome": "Arquivado"}, 201)
				portfolioRequest(t, handler, "DELETE", "/api/empreendimentos/1", nil, 204)
				id := int64(1)
				batch.Imoveis[1].EmpreendimentoID = &id
			case "duplicate":
				batch.Imoveis[1] = batch.Imoveis[0]
			case "invalid status":
				batch.Imoveis[1].Status = "ocupado"
			case "empty":
				batch.Imoveis = nil
			case "oversize":
				batch.Imoveis = make([]domain.Imovel, 101)
			case "negative cost":
				batch.CompraC = -1
			case "invalid mode":
				batch.ModoCusto = "outro"
			case "existing ID":
				batch.Imoveis[1].ID = 1
			}
			portfolioRequest(t, handler, "POST", "/api/imoveis/lote", batch, 422)
			var count int
			_ = db.QueryRow(`SELECT COUNT(*) FROM imoveis`).Scan(&count)
			if count != 0 {
				t.Fatalf("failed batch partially committed %d properties", count)
			}
		})
	}
}

func TestPropertyBatchFixedCosts(t *testing.T) {
	batch := sqlite.PropertyBatch{ModoCusto: "fixo", CompraC: 101, ReformaC: 3, OutrosC: 1, Imoveis: []domain.Imovel{portfolioProperty("A"), portfolioProperty("B")}}
	items, err := batch.Preview()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.ValorCompraC != 101 || item.InvestimentoTotalC != 105 {
			t.Fatalf("fixed costs were split: %+v", item)
		}
	}
}

func TestProjectsCRUDArchiveAndProtectedDeletion(t *testing.T) {
	db := testDB(t)
	handler := (&Server{Store: &sqlite.Store{DB: db}}).Handler()
	portfolioRequest(t, handler, "POST", "/api/empreendimentos", map[string]string{"nome": "Portugal"}, 201)
	portfolioRequest(t, handler, "POST", "/api/empreendimentos", map[string]string{"nome": "Portugal"}, 409)
	property := portfolioProperty("Portugal 620-1")
	id := int64(1)
	property.EmpreendimentoID = &id
	property.MatriculaCartorio = "10.001"
	portfolioRequest(t, handler, "POST", "/api/imoveis", property, 201)
	portfolioRequest(t, handler, "PATCH", "/api/empreendimentos/1", map[string]any{"nome": "Portugal atualizado", "endereco": "Rua Portugal", "unidade_ids": []int64{1}}, 200)
	if _, err := db.Exec(`UPDATE empreendimento_unidades SET peso=7 WHERE empreendimento_id=1`); err != nil {
		t.Fatal(err)
	}
	portfolioRequest(t, handler, "PATCH", "/api/empreendimentos/1", map[string]any{"nome": "Portugal atualizado", "unidade_ids": []int64{1}}, 200)
	portfolioRequest(t, handler, "PATCH", "/api/empreendimentos/1", map[string]any{"nome": "Somente nome"}, 200)
	var weight int
	_ = db.QueryRow(`SELECT peso FROM empreendimento_unidades WHERE empreendimento_id=1 AND imovel_id=1`).Scan(&weight)
	if weight != 7 {
		t.Fatal("project edits lost existing allocation weights/default units")
	}
	portfolioRequest(t, handler, "PATCH", "/api/empreendimentos/1", map[string]any{"nome": "Não deve ser salvo", "unidade_ids": []int64{999}}, 422)
	var name string
	if err := db.QueryRow(`SELECT nome FROM empreendimentos WHERE id=1`).Scan(&name); err != nil || name != "Somente nome" {
		t.Fatalf("invalid default allocation partially changed the project: %s %v", name, err)
	}
	portfolioRequest(t, handler, "DELETE", "/api/empreendimentos/1/permanente", nil, 409)
	portfolioRequest(t, handler, "DELETE", "/api/empreendimentos/1", nil, 204)
	active := portfolioRequest(t, handler, "GET", "/api/empreendimentos", nil, 200)
	if active.Body.String() != "[]\n" {
		t.Fatal("archived project leaked into active selector")
	}
	all := portfolioRequest(t, handler, "GET", "/api/empreendimentos?incluir_inativos=true", nil, 200)
	var projects []domain.Empreendimento
	_ = json.Unmarshal(all.Body.Bytes(), &projects)
	if len(projects) != 1 || projects[0].Ativo || len(projects[0].UnidadeIDs) != 1 {
		t.Fatalf("archive lost project relationships: %+v", projects)
	}
	portfolioRequest(t, handler, "POST", "/api/empreendimentos/1/reativar", nil, 200)
	portfolioRequest(t, handler, "POST", "/api/empreendimentos/999/reativar", nil, 404)
	portfolioRequest(t, handler, "DELETE", "/api/empreendimentos/999/permanente", nil, 404)
	portfolioRequest(t, handler, "POST", "/api/empreendimentos", map[string]string{"nome": "Sem vínculos"}, 201)
	portfolioRequest(t, handler, "DELETE", "/api/empreendimentos/2/permanente", nil, 204)
	portfolioRequest(t, handler, "GET", "/api/empreendimentos/2", nil, 404)
	var projectID int64
	var registry string
	_ = db.QueryRow(`SELECT empreendimento_id,matricula_cartorio FROM imoveis WHERE id=1`).Scan(&projectID, &registry)
	if projectID != 1 || registry != "10.001" {
		t.Fatal("project CRUD changed the property membership/registry")
	}
}

func TestChangingProjectDefaultsPreservesHistoricalAllocations(t *testing.T) {
	db := testDB(t)
	handler := (&Server{Store: &sqlite.Store{DB: db}}).Handler()
	projectID := int64(1)
	portfolioRequest(t, handler, "POST", "/api/imoveis", portfolioProperty("Unidade"), 201)
	portfolioRequest(t, handler, "POST", "/api/empreendimentos", map[string]any{"nome": "Obra", "unidade_ids": []int64{1}}, 201)
	portfolioRequest(t, handler, "POST", "/api/movimentacoes", map[string]any{
		"movimento": domain.Movimento{Tipo: "despesa", Categoria: "obra", Natureza: "investimento", Data: "2026-01-01", ValorC: 101, EmpreendimentoID: &projectID},
	}, 201)
	portfolioRequest(t, handler, "PATCH", "/api/empreendimentos/1", map[string]any{"nome": "Obra editada", "unidade_ids": []int64{}}, 200)
	var allocated, investment int64
	if err := db.QueryRow(`SELECT valor_c FROM movimentacao_rateios WHERE imovel_id=1`).Scan(&allocated); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT investimento_importado_c FROM imoveis WHERE id=1`).Scan(&investment); err != nil {
		t.Fatal(err)
	}
	if allocated != 101 || investment != 101 {
		t.Fatalf("project edit rewrote financial history: allocation=%d investment=%d", allocated, investment)
	}
}

func TestProjectDeletionProtectsAllRelationshipKinds(t *testing.T) {
	for _, relationship := range []string{"property", "allocation default", "expense", "movement"} {
		t.Run(relationship, func(t *testing.T) {
			db := testDB(t)
			handler := (&Server{Store: &sqlite.Store{DB: db}}).Handler()
			portfolioRequest(t, handler, "POST", "/api/empreendimentos", map[string]string{"nome": "Preservado"}, 201)
			portfolioRequest(t, handler, "POST", "/api/imoveis", portfolioProperty("Unidade"), 201)
			query := map[string]string{
				"property":           `UPDATE imoveis SET empreendimento_id=1,ativo=0 WHERE id=1`,
				"allocation default": `INSERT INTO empreendimento_unidades(empreendimento_id,imovel_id) VALUES(1,1)`,
				"expense":            `INSERT INTO despesas(imovel_id,data,categoria,valor_c,empreendimento_id) VALUES(1,'2026-01-01','outros',100,1)`,
				"movement":           `INSERT INTO movimentacoes(tipo,categoria,data,valor_c,empreendimento_id) VALUES('despesa','outros','2026-01-01',100,1)`,
			}[relationship]
			if _, err := db.Exec(query); err != nil {
				t.Fatal(err)
			}
			portfolioRequest(t, handler, "DELETE", "/api/empreendimentos/1/permanente", nil, 409)
			portfolioRequest(t, handler, "GET", "/api/empreendimentos/1", nil, 200)
		})
	}
}
