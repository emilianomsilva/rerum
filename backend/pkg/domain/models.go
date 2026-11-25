package domain

type Imovel struct {
	ID                     int64   `json:"id"`
	Apelido                string  `json:"apelido"`
	Endereco               string  `json:"endereco"`
	Tipo                   string  `json:"tipo"`
	Status                 string  `json:"status"`
	ValorCompraC           int64   `json:"valor_compra_c"`
	ReformaC               int64   `json:"reforma_c"`
	ITBIC                  int64   `json:"itbi_c"`
	CorretagemC            int64   `json:"corretagem_c"`
	OutrosCustosC          int64   `json:"outros_custos_c"`
	DataAquisicao          *string `json:"data_aquisicao"`
	Observacao             string  `json:"observacao"`
	Ativo                  bool    `json:"ativo"`
	MatriculaCartorio      string  `json:"matricula_cartorio"`
	InscricaoIPTU          string  `json:"inscricao_iptu"`
	UnidadeAgua            string  `json:"unidade_agua"`
	UnidadeEnergia         string  `json:"unidade_energia"`
	EmpreendimentoID       *int64  `json:"empreendimento_id"`
	Bairro                 string  `json:"bairro"`
	SituacaoPatrimonial    string  `json:"situacao_patrimonial"`
	InvestimentoImportadoC int64   `json:"investimento_importado_c"`
	InvestimentoTotalC     int64   `json:"investimento_total_c"`
	DestinadoLocacao       bool    `json:"destinado_locacao"`
	MatriculaCompartilhada bool    `json:"matricula_compartilhada"`
	DataVenda              *string `json:"data_venda"`
	ValorVendaC            int64   `json:"valor_venda_c"`
	Numero                 string  `json:"numero"`
	Unidade                string  `json:"unidade"`
}

func (p Imovel) Investimento() int64 {
	return p.ValorCompraC + p.ReformaC + p.ITBIC + p.CorretagemC + p.OutrosCustosC + p.InvestimentoImportadoC
}

type Inquilino struct {
	ID            int64  `json:"id"`
	Nome          string `json:"nome"`
	CPF           string `json:"cpf"`
	Telefone      string `json:"telefone"`
	Email         string `json:"email"`
	Observacao    string `json:"observacao"`
	Ativo         bool   `json:"ativo"`
	RG            string `json:"rg"`
	RendaMensalC  int64  `json:"renda_mensal_c"`
	LocalTrabalho string `json:"local_trabalho"`
	EnderecoAtual string `json:"endereco_atual"`
}

type Locacao struct {
	ID                 int64   `json:"id"`
	ImovelID           int64   `json:"imovel_id"`
	InquilinoID        int64   `json:"inquilino_id"`
	Inicio             string  `json:"inicio"`
	Fim                string  `json:"fim"`
	DiaVencimento      int     `json:"dia_vencimento"`
	ValorMensalC       int64   `json:"valor_mensal_c"`
	MultaPct           float64 `json:"multa_pct"`
	JurosDiaPct        float64 `json:"juros_dia_pct"`
	ReajusteIndice     string  `json:"reajuste_indice"`
	Status             string  `json:"status"`
	Intermediario      string  `json:"intermediario"`
	Observacao         string  `json:"observacao"`
	PrimeiroVencimento *string `json:"primeiro_vencimento"`
}

type LeaseSummary struct {
	Locacao
	Imovel        string  `json:"imovel"`
	Inquilino     string  `json:"inquilino"`
	Parcelas      int     `json:"parcelas"`
	PeriodoInicio *string `json:"periodo_inicio"`
	PeriodoFim    *string `json:"periodo_fim"`
}

type Parcela struct {
	ID            int64   `json:"id"`
	LocacaoID     int64   `json:"locacao_id"`
	Competencia   string  `json:"competencia"`
	Vencimento    string  `json:"vencimento"`
	ValorBaseC    int64   `json:"valor_base_c"`
	MultaC        int64   `json:"multa_c"`
	JurosC        int64   `json:"juros_c"`
	DescontoC     int64   `json:"desconto_c"`
	ValorPagoC    int64   `json:"valor_pago_c"`
	DataPagamento *string `json:"data_pagamento"`
	Forma         *string `json:"forma"`
	Status        string  `json:"status"`
	ReciboPath    *string `json:"recibo_path"`
	Numero        int     `json:"numero"`
	TotalParcelas int     `json:"total_parcelas"`
	PeriodoInicio *string `json:"periodo_inicio"`
	PeriodoFim    *string `json:"periodo_fim"`
	Tipo          string  `json:"tipo"`
	Observacao    string  `json:"observacao"`
	Imovel        string  `json:"imovel,omitempty"`
	Inquilino     string  `json:"inquilino,omitempty"`
	TotalDevidoC  int64   `json:"total_devido_c,omitempty"`
	SaldoC        int64   `json:"saldo_c,omitempty"`
	Situacao      string  `json:"situacao,omitempty"`
	TemRecibo     bool    `json:"tem_recibo,omitempty"`
}

type Movimento struct {
	ID               int64  `json:"id"`
	Tipo             string `json:"tipo"`
	Categoria        string `json:"categoria"`
	Data             string `json:"data"`
	ValorC           int64  `json:"valor_c"`
	Descricao        string `json:"descricao"`
	ImovelID         *int64 `json:"imovel_id"`
	EmpreendimentoID *int64 `json:"empreendimento_id"`
	InquilinoID      *int64 `json:"inquilino_id"`
	LocacaoID        *int64 `json:"locacao_id"`
	ParcelaID        *int64 `json:"parcela_id"`
	Referencia       string `json:"referencia"`
	FormaPagamento   string `json:"forma_pagamento"`
	Origem           string `json:"origem"`
	Fornecedor       string `json:"fornecedor"`
	Observacao       string `json:"observacao"`
	Reversao         bool   `json:"reversao"`
	Natureza         string `json:"natureza"`
	Imovel           string `json:"imovel,omitempty"`
	Inquilino        string `json:"inquilino,omitempty"`
}

type Rateio struct {
	ImovelID int64 `json:"imovel_id"`
	ValorC   int64 `json:"valor_c"`
}

type Despesa struct {
	ID               int64  `json:"id"`
	ImovelID         int64  `json:"imovel_id"`
	Data             string `json:"data"`
	Categoria        string `json:"categoria"`
	ValorC           int64  `json:"valor_c"`
	Descricao        string `json:"descricao"`
	Fornecedor       string `json:"fornecedor"`
	FormaPagamento   string `json:"forma_pagamento"`
	EmpreendimentoID *int64 `json:"empreendimento_id"`
	Imovel           string `json:"imovel,omitempty"`
}

type Empreendimento struct {
	ID         int64   `json:"id"`
	Nome       string  `json:"nome"`
	Endereco   string  `json:"endereco"`
	Observacao string  `json:"observacao"`
	Ativo      bool    `json:"ativo"`
	UnidadeIDs []int64 `json:"unidade_ids"`
}

type Divida struct {
	ID              int64   `json:"id"`
	Credor          string  `json:"credor"`
	Descricao       string  `json:"descricao"`
	ValorOriginalC  int64   `json:"valor_original_c"`
	DataContratacao string  `json:"data_contratacao"`
	Vencimento      *string `json:"vencimento"`
	Observacao      string  `json:"observacao"`
	Ativa           bool    `json:"ativa"`
}

type MovimentoDivida struct {
	ID             int64  `json:"id"`
	DividaID       int64  `json:"divida_id"`
	Tipo           string `json:"tipo"`
	Data           string `json:"data"`
	ValorC         int64  `json:"valor_c"`
	JurosC         int64  `json:"juros_c"`
	FormaPagamento string `json:"forma_pagamento"`
	Descricao      string `json:"descricao"`
}

type Roi struct {
	ImovelID        int64   `json:"imovel_id"`
	Apelido         string  `json:"apelido"`
	InvestimentoC   int64   `json:"investimento_c"`
	RecebidoTotalC  int64   `json:"recebido_total_c"`
	DespesasTotalC  int64   `json:"despesas_total_c"`
	Recebido12mC    int64   `json:"recebido_12m_c"`
	Despesas12mC    int64   `json:"despesas_12m_c"`
	ROIAnualPct     float64 `json:"roi_anual_pct"`
	ROIAcumuladoPct float64 `json:"roi_acumulado_pct"`
}

type DashboardPayment struct {
	ID            int64  `json:"id"`
	Numero        int    `json:"numero"`
	TotalParcelas int    `json:"total_parcelas"`
	Vencimento    string `json:"vencimento"`
	Imovel        string `json:"imovel"`
	Inquilino     string `json:"inquilino"`
	SaldoC        int64  `json:"saldo_c"`
}

type Dashboard struct {
	SerieMensal              []MonthlyCashflow  `json:"serie_mensal"`
	TotalImoveis             int                `json:"total_imoveis"`
	Ocupados                 int                `json:"ocupados"`
	Disponiveis              int                `json:"disponiveis"`
	Manutencao               int                `json:"manutencao"`
	TaxaOcupacaoPct          float64            `json:"taxa_ocupacao_pct"`
	ReceitaMesC              int64              `json:"receita_mes_c"`
	AReceberMesC             int64              `json:"a_receber_mes_c"`
	DespesasOperacionaisMesC int64              `json:"despesas_operacionais_mes_c"`
	ResultadoOperacionalMesC int64              `json:"resultado_operacional_mes_c"`
	CapitalInvestidoC        int64              `json:"capital_investido_c"`
	ROICarteira12mPct        *float64           `json:"roi_carteira_12m_pct"`
	VencidasQtd              int                `json:"vencidas_qtd"`
	VencidasC                int64              `json:"vencidas_c"`
	Vencidas                 []DashboardPayment `json:"vencidas"`
	Proximas                 []DashboardPayment `json:"proximas"`
	ROIs                     []Roi              `json:"rois"`
}

type MonthlyCashflow struct {
	Mes                      string `json:"mes"`
	ReceitasC                int64  `json:"receitas_c"`
	DespesasOperacionaisC    int64  `json:"despesas_operacionais_c"`
	ResultadoOperacionalC    int64  `json:"resultado_operacional_c"`
	ReceitasSemDetalhamentoC int64  `json:"receitas_sem_detalhamento_c"`
}

type StatisticsLine struct {
	InvestimentosC        int64   `json:"investimentos_c"`
	ImovelID              int64   `json:"imovel_id"`
	Apelido               string  `json:"apelido"`
	CustoC                int64   `json:"custo_c"`
	RendimentoC           int64   `json:"rendimento_c"`
	DespesasOperacionaisC int64   `json:"despesas_operacionais_c"`
	ResultadoLiquidoC     int64   `json:"resultado_liquido_c"`
	ROIPct                float64 `json:"roi_pct"`
	ROILiquidoPct         float64 `json:"roi_liquido_pct"`
	DestinadoLocacao      bool    `json:"destinado_locacao"`
}

type Statistics struct {
	InvestimentosTotalC        int64            `json:"investimentos_total_c"`
	Linhas                     []StatisticsLine `json:"linhas"`
	CustoTotalC                int64            `json:"custo_total_c"`
	RendimentoTotalC           int64            `json:"rendimento_total_c"`
	DespesasOperacionaisTotalC int64            `json:"despesas_operacionais_total_c"`
	ResultadoLiquidoTotalC     int64            `json:"resultado_liquido_total_c"`
	ROITotalPct                float64          `json:"roi_total_pct"`
	ROILiquidoTotalPct         float64          `json:"roi_liquido_total_pct"`
}

type AnnualReportMonth struct {
	Mes                   string `json:"mes"`
	ReceitasC             int64  `json:"receitas_c"`
	DespesasOperacionaisC int64  `json:"despesas_operacionais_c"`
	InvestimentosC        int64  `json:"investimentos_c"`
	ResultadoLiquidoC     int64  `json:"resultado_liquido_c"`
}

type AnnualReportTenantItem struct {
	InquilinoNome  string `json:"inquilino_nome"`
	InquilinoCPF   string `json:"inquilino_cpf"`
	ImovelApelido  string `json:"imovel_apelido"`
	TotalRecebidoC int64  `json:"total_recebido_c"`
}

type AnalyticsCategory struct {
	Chave  string `json:"chave"`
	Rotulo string `json:"rotulo"`
}

type AnalyticsSeries struct {
	Chave    string  `json:"chave"`
	Rotulo   string  `json:"rotulo"`
	ValoresC []int64 `json:"valores_c"`
}

type AnalyticsResult struct {
	Categorias []AnalyticsCategory `json:"categorias"`
	Series     []AnalyticsSeries   `json:"series"`
	Avisos     []string            `json:"avisos"`
}

type AnnualReport struct {
	Ano                 int                      `json:"ano"`
	Meses               []AnnualReportMonth      `json:"meses"`
	Inquilinos          []AnnualReportTenantItem `json:"inquilinos"`
	TotalReceitasC      int64                    `json:"total_receitas_c"`
	TotalDespesasOpexC  int64                    `json:"total_despesas_opex_c"`
	TotalInvestimentosC int64                    `json:"total_investimentos_c"`
	ResultadoLiquidoC   int64                    `json:"resultado_liquido_c"`
}
