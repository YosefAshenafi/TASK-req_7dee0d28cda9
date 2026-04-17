package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
	"github.com/fulfillops/fulfillops/internal/util"
	"github.com/fulfillops/fulfillops/internal/view"
	cview "github.com/fulfillops/fulfillops/internal/view/customers"
)

type PageCustomerHandler struct {
	store        sessions.Store
	customerRepo repository.CustomerRepository
	fulfillRepo  repository.FulfillmentRepository
	encSvc       service.EncryptionService
	auditSvc     service.AuditService
}

func NewPageCustomerHandler(
	store sessions.Store,
	customerRepo repository.CustomerRepository,
	fulfillRepo repository.FulfillmentRepository,
	encSvc service.EncryptionService,
) *PageCustomerHandler {
	return &PageCustomerHandler{store: store, customerRepo: customerRepo, fulfillRepo: fulfillRepo, encSvc: encSvc}
}

func (h *PageCustomerHandler) WithAudit(auditSvc service.AuditService) *PageCustomerHandler {
	h.auditSvc = auditSvc
	return h
}

func (h *PageCustomerHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	q := c.Query("q")
	page := queryInt(c, "page", 1)
	const size = 20

	customers, total, _ := h.customerRepo.List(ctx, q, domain.PageRequest{Page: page, PageSize: size}, false)

	masked := make([]cview.MaskedCustomer, len(customers))
	for i, cu := range customers {
		mc := cview.MaskedCustomer{Customer: cu}
		if cu.PhoneEncrypted != nil {
			if plain, err := h.encSvc.DecryptToString(cu.PhoneEncrypted); err == nil {
				mc.PhoneMasked = util.MaskPhone(plain)
			}
		}
		if cu.EmailEncrypted != nil {
			if plain, err := h.encSvc.DecryptToString(cu.EmailEncrypted); err == nil {
				mc.EmailMasked = util.MaskEmail(plain)
			}
		}
		masked[i] = mc
	}

	renderPage(c, http.StatusOK, cview.List(pageCtx(c, h.store), cview.ListData{
		Customers: masked,
		Query:     q,
		Pager:     view.NewPagination(page, size, total, "/customers", "q="+q),
		CanEdit:   canEdit(c, h.store),
	}))
}

func (h *PageCustomerHandler) ShowDetail(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	cu, err := h.customerRepo.GetByID(ctx, id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	phoneMasked, emailMasked, addrMasked := "", "", ""
	if cu.PhoneEncrypted != nil {
		if plain, err2 := h.encSvc.DecryptToString(cu.PhoneEncrypted); err2 == nil {
			phoneMasked = util.MaskPhone(plain)
		}
	}
	if cu.EmailEncrypted != nil {
		if plain, err2 := h.encSvc.DecryptToString(cu.EmailEncrypted); err2 == nil {
			emailMasked = util.MaskEmail(plain)
		}
	}
	if cu.AddressEncrypted != nil {
		if plain, err2 := h.encSvc.DecryptToString(cu.AddressEncrypted); err2 == nil {
			addrMasked = util.MaskAddress(plain)
		}
	}

	fulfillments, _, _ := h.fulfillRepo.List(ctx, repository.FulfillmentFilters{CustomerID: &id},
		domain.PageRequest{Page: 1, PageSize: 20})

	renderPage(c, http.StatusOK, cview.Detail(pageCtx(c, h.store), cview.DetailData{
		Customer:      *cu,
		PhoneMasked:   phoneMasked,
		EmailMasked:   emailMasked,
		AddressMasked: addrMasked,
		Fulfillments:  fulfillments,
		CanEdit:       canEdit(c, h.store),
	}))
}

func (h *PageCustomerHandler) ShowCreate(c *gin.Context) {
	renderPage(c, http.StatusOK, cview.CustomerForm(pageCtx(c, h.store), cview.CustomerFormData{}))
}

func (h *PageCustomerHandler) ShowEdit(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	cu, err := h.customerRepo.GetByID(ctx, id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	phone, email, addr := "", "", ""
	if cu.PhoneEncrypted != nil {
		phone, _ = h.encSvc.DecryptToString(cu.PhoneEncrypted)
	}
	if cu.EmailEncrypted != nil {
		email, _ = h.encSvc.DecryptToString(cu.EmailEncrypted)
	}
	if cu.AddressEncrypted != nil {
		addr, _ = h.encSvc.DecryptToString(cu.AddressEncrypted)
	}
	renderPage(c, http.StatusOK, cview.CustomerForm(pageCtx(c, h.store), cview.CustomerFormData{
		Customer: cu, Phone: phone, Email: email, Address: addr,
	}))
}

func (h *PageCustomerHandler) PostCreate(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	cu := &domain.Customer{Name: c.PostForm("name")}
	if p := c.PostForm("phone"); p != "" {
		enc, err := h.encSvc.Encrypt([]byte(p))
		if err != nil {
			redirectWithFlash(c, h.store, "/customers/new", "error", "Encryption failed.")
			return
		}
		cu.PhoneEncrypted = enc
	}
	if e := c.PostForm("email"); e != "" {
		enc, err := h.encSvc.Encrypt([]byte(e))
		if err != nil {
			redirectWithFlash(c, h.store, "/customers/new", "error", "Encryption failed.")
			return
		}
		cu.EmailEncrypted = enc
	}
	if addr := buildAddressStr(c); addr != "" {
		enc, err := h.encSvc.Encrypt([]byte(addr))
		if err != nil {
			redirectWithFlash(c, h.store, "/customers/new", "error", "Encryption failed.")
			return
		}
		cu.AddressEncrypted = enc
	}
	created, err := h.customerRepo.Create(ctx, cu)
	if err != nil {
		redirectWithFlash(c, h.store, "/customers/new", "error", err.Error())
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Log(ctx, "customers", created.ID, "CREATE", nil, created)
	}
	redirectWithFlash(c, h.store, "/customers", "success", "Customer created.")
}

func (h *PageCustomerHandler) PostUpdate(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	id, _ := uuid.Parse(c.Param("id"))

	existing, err := h.customerRepo.GetByID(ctx, id)
	if err != nil {
		redirectWithFlash(c, h.store, "/customers/"+id.String()+"/edit", "error", "Customer not found.")
		return
	}

	cu := &domain.Customer{
		ID:      id,
		Name:    c.PostForm("name"),
		Version: formInt(c, "version"),
	}

	if p := c.PostForm("phone"); p != "" {
		enc, err := h.encSvc.Encrypt([]byte(p))
		if err != nil {
			redirectWithFlash(c, h.store, "/customers/"+id.String()+"/edit", "error", "Encryption failed.")
			return
		}
		cu.PhoneEncrypted = enc
	} else {
		cu.PhoneEncrypted = existing.PhoneEncrypted
	}

	if e := c.PostForm("email"); e != "" {
		enc, err := h.encSvc.Encrypt([]byte(e))
		if err != nil {
			redirectWithFlash(c, h.store, "/customers/"+id.String()+"/edit", "error", "Encryption failed.")
			return
		}
		cu.EmailEncrypted = enc
	} else {
		cu.EmailEncrypted = existing.EmailEncrypted
	}

	if addr := buildAddressStr(c); addr != "" {
		enc, err := h.encSvc.Encrypt([]byte(addr))
		if err != nil {
			redirectWithFlash(c, h.store, "/customers/"+id.String()+"/edit", "error", "Encryption failed.")
			return
		}
		cu.AddressEncrypted = enc
	} else {
		cu.AddressEncrypted = existing.AddressEncrypted
	}

	updated, err := h.customerRepo.Update(ctx, cu)
	if err != nil {
		redirectWithFlash(c, h.store, "/customers/"+id.String()+"/edit", "error", err.Error())
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Log(ctx, "customers", updated.ID, "UPDATE", existing, updated)
	}
	redirectWithFlash(c, h.store, "/customers/"+id.String(), "success", "Customer updated.")
}

func (h *PageCustomerHandler) PostDelete(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	id, _ := uuid.Parse(c.Param("id"))
	sess, _ := h.store.Get(c.Request, "fulfillops")
	deletedBy, _ := uuid.Parse(sess.Values["userID"].(string))
	before, _ := h.customerRepo.GetByID(ctx, id)
	if err := h.customerRepo.SoftDelete(ctx, id, deletedBy); err != nil {
		redirectWithFlash(c, h.store, "/customers/"+id.String(), "error", "Delete failed: "+err.Error())
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Log(ctx, "customers", id, "DELETE", before, map[string]any{"deleted_by": deletedBy})
	}
	redirectWithFlash(c, h.store, "/customers", "success", "Customer deleted.")
}

func (h *PageCustomerHandler) PostRestore(c *gin.Context) {
	ctx := pageRequestContextWithUser(c, h.store)
	id, _ := uuid.Parse(c.Param("id"))
	if err := h.customerRepo.Restore(ctx, id); err != nil {
		redirectWithFlash(c, h.store, "/admin/recovery", "error", "Restore failed.")
		return
	}
	if h.auditSvc != nil {
		if restored, err := h.customerRepo.GetByID(ctx, id); err == nil {
			_ = h.auditSvc.Log(ctx, "customers", id, "RESTORE", nil, restored)
		}
	}
	redirectWithFlash(c, h.store, "/admin/recovery", "success", "Customer restored.")
}

func buildAddressStr(c *gin.Context) string {
	parts := []string{
		c.PostForm("address_line1"),
		c.PostForm("address_line2"),
		c.PostForm("city"),
		c.PostForm("state"),
		c.PostForm("zip_code"),
	}
	var result []string
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return strings.Join(result, "|")
}
