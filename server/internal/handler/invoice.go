package handler

import (
	"fmt"
	"net/http"
	"time"

	"hotel-management/server/internal/db"
)

// ============================================================
// 发票管理（Invoice）
//   * 发票抬头：个人/企业，可关联客户档案，支持多抬头与默认抬头
//   * 发票开具：关联账单(folio)，系统自动生成发票号（FP+年份+8位流水）
//   * 发票状态：0待开 1已开 2作废 3红冲
//   * 演示环境不对接真实税控，仅做业务流程与台账管理
// ============================================================

// ---------------- 发票抬头 ----------------

// ListInvoiceTitles 发票抬头列表（GET /api/v1/invoice-titles?customer_id=）。
func ListInvoiceTitles(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	customerID := queryInt64(r, "customer_id")
	query := `SELECT id, COALESCE(customer_id,0), title_type, title_name, COALESCE(tax_no,''),
	                 COALESCE(address,''), COALESCE(phone,''), COALESCE(bank_name,''), COALESCE(bank_account,''),
	                 COALESCE(email,''), is_default, created_at
	          FROM invoice_title`
	args := []any{}
	if customerID > 0 {
		query += " WHERE customer_id = $1"
		args = append(args, customerID)
	}
	query += " ORDER BY is_default DESC, id DESC"

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type title struct {
		ID          int64     `json:"id"`
		CustomerID  int64     `json:"customer_id"`
		TitleType   int       `json:"title_type"`
		TitleName   string    `json:"title_name"`
		TaxNo       string    `json:"tax_no"`
		Address     string    `json:"address"`
		Phone       string    `json:"phone"`
		BankName    string    `json:"bank_name"`
		BankAccount string    `json:"bank_account"`
		Email       string    `json:"email"`
		IsDefault   int       `json:"is_default"`
		CreatedAt   time.Time `json:"created_at"`
	}
	list := make([]title, 0)
	for rows.Next() {
		var t title
		if err := rows.Scan(&t.ID, &t.CustomerID, &t.TitleType, &t.TitleName, &t.TaxNo,
			&t.Address, &t.Phone, &t.BankName, &t.BankAccount, &t.Email, &t.IsDefault, &t.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{"titles": list, "total": len(list)})
}

// CreateInvoiceTitle 新增发票抬头（POST /api/v1/invoice-titles）。
func CreateInvoiceTitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		CustomerID  int64  `json:"customer_id"`
		TitleType   int    `json:"title_type"`
		TitleName   string `json:"title_name"`
		TaxNo       string `json:"tax_no"`
		Address     string `json:"address"`
		Phone       string `json:"phone"`
		BankName    string `json:"bank_name"`
		BankAccount string `json:"bank_account"`
		Email       string `json:"email"`
		IsDefault   int    `json:"is_default"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.TitleName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "抬头名称不能为空"})
		return
	}
	if req.TitleType == 1 && req.TaxNo == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "企业抬头需填写税号"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	tx, err := pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer tx.Rollback(r.Context())

	// 设为默认时，先清除同客户的其他默认抬头
	if req.IsDefault == 1 && req.CustomerID > 0 {
		if _, err := tx.Exec(r.Context(), `UPDATE invoice_title SET is_default = 0 WHERE customer_id = $1`, req.CustomerID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	var custArg any = req.CustomerID
	if req.CustomerID == 0 {
		custArg = nil
	}
	var id int64
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO invoice_title (customer_id, title_type, title_name, tax_no, address, phone, bank_name, bank_account, email, is_default)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
		custArg, req.TitleType, req.TitleName, req.TaxNo, req.Address, req.Phone, req.BankName, req.BankAccount, req.Email, req.IsDefault,
	).Scan(&id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

// UpdateInvoiceTitle 修改发票抬头（PUT /api/v1/invoice-titles/{id}）。
func UpdateInvoiceTitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := pathValueInt64(r, "id")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少抬头 ID"})
		return
	}
	var req struct {
		TitleType   int    `json:"title_type"`
		TitleName   string `json:"title_name"`
		TaxNo       string `json:"tax_no"`
		Address     string `json:"address"`
		Phone       string `json:"phone"`
		BankName    string `json:"bank_name"`
		BankAccount string `json:"bank_account"`
		Email       string `json:"email"`
		IsDefault   *int   `json:"is_default"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.TitleName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "抬头名称不能为空"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	tx, err := pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer tx.Rollback(r.Context())

	// 设为默认时清除同客户其他默认
	if req.IsDefault != nil && *req.IsDefault == 1 {
		var custID int64
		_ = tx.QueryRow(r.Context(), `SELECT COALESCE(customer_id,0) FROM invoice_title WHERE id = $1`, id).Scan(&custID)
		if custID > 0 {
			if _, err := tx.Exec(r.Context(), `UPDATE invoice_title SET is_default = 0 WHERE customer_id = $1`, custID); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
	}

	isDefault := 0
	if req.IsDefault != nil {
		isDefault = *req.IsDefault
	}
	if _, err := tx.Exec(r.Context(),
		`UPDATE invoice_title SET title_type=$1, title_name=$2, tax_no=$3, address=$4, phone=$5,
		        bank_name=$6, bank_account=$7, email=$8, is_default=$9, updated_at=now() WHERE id=$10`,
		req.TitleType, req.TitleName, req.TaxNo, req.Address, req.Phone, req.BankName, req.BankAccount, req.Email, isDefault, id,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "ok": true})
}

// DeleteInvoiceTitle 删除发票抬头（DELETE /api/v1/invoice-titles/{id}）。
func DeleteInvoiceTitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := pathValueInt64(r, "id")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少抬头 ID"})
		return
	}
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	if _, err := pool.Exec(r.Context(), `DELETE FROM invoice_title WHERE id = $1`, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

// ---------------- 发票记录 ----------------

// ListInvoices 发票列表（GET /api/v1/invoices）。
// 过滤：store_id, status, keyword(发票号/抬头), start_date, end_date。
func ListInvoices(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	storeID := queryInt64(r, "store_id")
	status := -1
	if s := r.URL.Query().Get("status"); s != "" {
		fmt.Sscanf(s, "%d", &status)
	}
	keyword := r.URL.Query().Get("keyword")
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")
	u := currentUser(r)

	query := `SELECT i.id, i.store_id, s.name, i.invoice_no, COALESCE(i.folio_id,0), COALESCE(i.check_in_id,0),
	                 COALESCE(i.customer_id,0), COALESCE(i.title_id,0), i.invoice_type, i.title_name, COALESCE(i.tax_no,''),
	                 i.amount, i.tax_amount, i.status, COALESCE(ub.name,''), i.issued_at, i.created_at, COALESCE(i.remark,'')
	          FROM invoice i
	          JOIN store s ON s.id = i.store_id
	          LEFT JOIN users ub ON ub.id = i.issued_by
	          WHERE 1=1`
	args := []any{}
	idx := 1

	if storeID > 0 {
		if u != nil && !u.IsAdmin && !u.canAccessStore(storeID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
			return
		}
		query += fmt.Sprintf(" AND i.store_id = $%d", idx)
		args = append(args, storeID)
		idx++
	} else if u != nil && !u.IsAdmin {
		if len(u.StoreIDs) == 0 {
			query += " AND FALSE"
		} else {
			query += fmt.Sprintf(" AND i.store_id = ANY($%d)", idx)
			args = append(args, u.StoreIDs)
			idx++
		}
	}
	if status >= 0 {
		query += fmt.Sprintf(" AND i.status = $%d", idx)
		args = append(args, status)
		idx++
	}
	if keyword != "" {
		query += fmt.Sprintf(" AND (i.invoice_no ILIKE $%d OR i.title_name ILIKE $%d)", idx, idx)
		args = append(args, "%"+keyword+"%")
		idx++
	}
	if startDate != "" {
		query += fmt.Sprintf(" AND i.created_at >= $%d::date", idx)
		args = append(args, startDate)
		idx++
	}
	if endDate != "" {
		query += fmt.Sprintf(" AND i.created_at < ($%d::date + INTERVAL '1 day')", idx)
		args = append(args, endDate)
		idx++
	}
	query += " ORDER BY i.created_at DESC LIMIT 300"

	rows, err := pool.Query(r.Context(), query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type inv struct {
		ID          int64      `json:"id"`
		StoreID     int64      `json:"store_id"`
		StoreName   string     `json:"store_name"`
		InvoiceNo   string     `json:"invoice_no"`
		FolioID     int64      `json:"folio_id"`
		CheckInID   int64      `json:"check_in_id"`
		CustomerID  int64      `json:"customer_id"`
		TitleID     int64      `json:"title_id"`
		InvoiceType int        `json:"invoice_type"`
		TitleName   string     `json:"title_name"`
		TaxNo       string     `json:"tax_no"`
		Amount      float64    `json:"amount"`
		TaxAmount   float64    `json:"tax_amount"`
		Status      int        `json:"status"`
		IssuedBy    string     `json:"issued_by"`
		IssuedAt    *time.Time `json:"issued_at"`
		CreatedAt   time.Time  `json:"created_at"`
		Remark      string     `json:"remark"`
	}
	list := make([]inv, 0)
	for rows.Next() {
		var it inv
		if err := rows.Scan(&it.ID, &it.StoreID, &it.StoreName, &it.InvoiceNo, &it.FolioID, &it.CheckInID,
			&it.CustomerID, &it.TitleID, &it.InvoiceType, &it.TitleName, &it.TaxNo,
			&it.Amount, &it.TaxAmount, &it.Status, &it.IssuedBy, &it.IssuedAt, &it.CreatedAt, &it.Remark); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		list = append(list, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{"invoices": list, "total": len(list)})
}

// CreateInvoice 开具发票（POST /api/v1/invoices）。
// 关联账单(folio)，金额取账单总额或指定金额；发票号系统自动生成。
func CreateInvoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var req struct {
		StoreID     int64   `json:"store_id"`
		FolioID     int64   `json:"folio_id"`
		TitleID     int64   `json:"title_id"`
		InvoiceType int     `json:"invoice_type"`
		Amount      float64 `json:"amount"`
		TaxAmount   float64 `json:"tax_amount"`
		Remark      string  `json:"remark"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.StoreID == 0 || req.TitleID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "门店和发票抬头不能为空"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(req.StoreID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}

	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}

	tx, err := pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer tx.Rollback(r.Context())

	// 取抬头信息（COALESCE 保证 customer_id 非 NULL）
	var titleName, taxNo string
	var titleType, titleCustID int64
	if err := tx.QueryRow(r.Context(),
		`SELECT title_name, COALESCE(tax_no,''), title_type, COALESCE(customer_id,0) FROM invoice_title WHERE id = $1`, req.TitleID,
	).Scan(&titleName, &taxNo, &titleType, &titleCustID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "发票抬头不存在"})
		return
	}
	if titleType == 1 && taxNo == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "企业抬头缺少税号"})
		return
	}

	// 金额：未指定则取账单已付金额
	amount := req.Amount
	var folioID, checkInID, customerFromFolio int64
	if req.FolioID > 0 {
		var paid float64
		if err := tx.QueryRow(r.Context(),
			`SELECT id, check_in_id, paid_amount FROM folio WHERE id = $1`, req.FolioID,
		).Scan(&folioID, &checkInID, &paid); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "账单不存在"})
			return
		}
		// 校验账单门店
		var folioStoreID int64
		_ = tx.QueryRow(r.Context(), `SELECT store_id FROM check_in WHERE id = $1`, checkInID).Scan(&folioStoreID)
		if folioStoreID != req.StoreID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "账单不属于该门店"})
			return
		}
		_ = tx.QueryRow(r.Context(), `SELECT customer_id FROM check_in WHERE id = $1`, checkInID).Scan(&customerFromFolio)
		if amount <= 0 {
			amount = paid
		}
	}
	if amount <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "开票金额必须大于 0"})
		return
	}

	taxAmount := req.TaxAmount
	if taxAmount < 0 {
		taxAmount = 0
	}

	// 生成发票号：FP + 年份 + 8位流水
	var seq int64
	_ = tx.QueryRow(r.Context(), `SELECT nextval('invoice_no_seq')`).Scan(&seq)
	invoiceNo := fmt.Sprintf("FP%s%08d", time.Now().Format("2006"), seq)

	opID := int64(0)
	if u := currentUser(r); u != nil {
		opID = u.ID
	}

	// 客户ID：优先取账单关联客户，其次抬头关联客户
	customerID := int64(0)
	if customerFromFolio > 0 {
		customerID = customerFromFolio
	} else if titleCustID > 0 {
		customerID = titleCustID
	}
	var folioArg, checkInArg, custArg any = nil, nil, nil
	if folioID > 0 {
		folioArg = folioID
	}
	if checkInID > 0 {
		checkInArg = checkInID
	}
	if customerID > 0 {
		custArg = customerID
	}

	var id int64
	if err := tx.QueryRow(r.Context(),
		`INSERT INTO invoice (store_id, invoice_no, folio_id, check_in_id, customer_id, title_id, invoice_type, title_name, tax_no, amount, tax_amount, status, issued_by, issued_at, remark)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 1, $12, now(), $13) RETURNING id`,
		req.StoreID, invoiceNo, folioArg, checkInArg, custArg, req.TitleID, req.InvoiceType, titleName, taxNo, amount, taxAmount, opID, req.Remark,
	).Scan(&id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         id,
		"invoice_no": invoiceNo,
		"amount":     amount,
		"tax_amount": taxAmount,
		"status":     1,
	})
	LogAction(w, r, req.StoreID, "invoice_create", invoiceNo, fmt.Sprintf("开具发票 ¥%.2f (%s)", amount, titleName))
}

// GetInvoice 发票详情（GET /api/v1/invoices/{id}）。
func GetInvoice(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	id := pathValueInt64(r, "id")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少发票 ID"})
		return
	}
	var (
		invoiceNo, titleName, taxNo, issuedBy, remark, storeName     string
		folioID, checkInID, customerID, titleID, invoiceType, status int64
		amount, taxAmount                                            float64
		issuedAt                                                     *time.Time
		createdAt                                                    time.Time
		storeID                                                      int64
	)
	if err := pool.QueryRow(r.Context(), `
		SELECT i.id, i.store_id, s.name, i.invoice_no, COALESCE(i.folio_id,0), COALESCE(i.check_in_id,0),
		       COALESCE(i.customer_id,0), COALESCE(i.title_id,0), i.invoice_type, i.title_name, COALESCE(i.tax_no,''),
		       i.amount, i.tax_amount, i.status, COALESCE(ub.name,''), i.issued_at, i.created_at, COALESCE(i.remark,'')
		FROM invoice i
		JOIN store s ON s.id = i.store_id
		LEFT JOIN users ub ON ub.id = i.issued_by
		WHERE i.id = $1`, id,
	).Scan(&id, &storeID, &storeName, &invoiceNo, &folioID, &checkInID, &customerID, &titleID,
		&invoiceType, &titleName, &taxNo, &amount, &taxAmount, &status, &issuedBy, &issuedAt, &createdAt, &remark); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "发票不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
		return
	}
	resp := map[string]any{
		"id":           id,
		"store_id":     storeID,
		"store_name":   storeName,
		"invoice_no":   invoiceNo,
		"folio_id":     folioID,
		"check_in_id":  checkInID,
		"customer_id":  customerID,
		"title_id":     titleID,
		"invoice_type": invoiceType,
		"title_name":   titleName,
		"tax_no":       taxNo,
		"amount":       amount,
		"tax_amount":   taxAmount,
		"status":       status,
		"issued_by":    issuedBy,
		"remark":       remark,
		"created_at":   createdAt.Format("2006-01-02 15:04:05"),
	}
	if issuedAt != nil {
		resp["issued_at"] = issuedAt.Format("2006-01-02 15:04:05")
	}
	writeJSON(w, http.StatusOK, resp)
}

// VoidInvoice 作废发票（POST /api/v1/invoices/{id}/void）。
// 仅已开(1)状态可作废。
func VoidInvoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := pathValueInt64(r, "id")
	if id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少发票 ID"})
		return
	}
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	var storeID, status int64
	var invoiceNo string
	if err := pool.QueryRow(r.Context(), `SELECT store_id, status, invoice_no FROM invoice WHERE id = $1`, id).Scan(&storeID, &status, &invoiceNo); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "发票不存在"})
		return
	}
	if u := currentUser(r); u != nil && !u.canAccessStore(storeID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权操作该门店"})
		return
	}
	if status != 1 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "仅已开发票可作废"})
		return
	}
	if _, err := pool.Exec(r.Context(),
		`UPDATE invoice SET status = 2, voided_at = now(), updated_at = now() WHERE id = $1`, id,
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": 2})
	LogAction(w, r, storeID, "invoice_void", invoiceNo, "作废发票")
}

// InvoiceSummary 发票汇总（GET /api/v1/invoices/summary?store_id=&start=&end=）。
func InvoiceSummary(w http.ResponseWriter, r *http.Request) {
	pool := db.Pool()
	if pool == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "数据库不可用"})
		return
	}
	storeID := queryInt64(r, "store_id")
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	if start == "" {
		start = time.Now().AddDate(0, 0, -29).Format("2006-01-02")
	}
	if end == "" {
		end = time.Now().Format("2006-01-02")
	}

	u := currentUser(r)
	where := "WHERE i.created_at::date BETWEEN $1::date AND $2::date AND i.status = 1"
	args := []any{start, end}
	idx := 3
	if storeID > 0 {
		if u != nil && !u.IsAdmin && !u.canAccessStore(storeID) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "无权访问该门店"})
			return
		}
		where += fmt.Sprintf(" AND i.store_id = $%d", idx)
		args = append(args, storeID)
		idx++
	} else if u != nil && !u.IsAdmin {
		if len(u.StoreIDs) == 0 {
			where += " AND FALSE"
		} else {
			where += fmt.Sprintf(" AND i.store_id = ANY($%d)", idx)
			args = append(args, u.StoreIDs)
			idx++
		}
	}

	var totalAmount, totalTax float64
	var totalCount int64
	_ = pool.QueryRow(r.Context(),
		`SELECT COALESCE(sum(amount),0), COALESCE(sum(tax_amount),0), count(*) FROM invoice i `+where, args...,
	).Scan(&totalAmount, &totalTax, &totalCount)

	// 按发票类型分组
	rows, err := pool.Query(r.Context(), `
		SELECT i.invoice_type, count(*), COALESCE(sum(i.amount),0)
		FROM invoice i `+where+`
		GROUP BY i.invoice_type ORDER BY i.invoice_type`, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	type typeStat struct {
		InvoiceType int     `json:"invoice_type"`
		Count       int64   `json:"count"`
		Amount      float64 `json:"amount"`
	}
	byType := make([]typeStat, 0)
	for rows.Next() {
		var t typeStat
		if err := rows.Scan(&t.InvoiceType, &t.Count, &t.Amount); err != nil {
			continue
		}
		byType = append(byType, t)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total_count":  totalCount,
		"total_amount": totalAmount,
		"total_tax":    totalTax,
		"by_type":      byType,
		"start":        start,
		"end":          end,
	})
}
