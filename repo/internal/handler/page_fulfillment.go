package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"

	"github.com/fulfillops/fulfillops/internal/domain"
	"github.com/fulfillops/fulfillops/internal/repository"
	"github.com/fulfillops/fulfillops/internal/service"
	"github.com/fulfillops/fulfillops/internal/util"
	"github.com/fulfillops/fulfillops/internal/view"
	fview "github.com/fulfillops/fulfillops/internal/view/fulfillments"
)

type PageFulfillmentHandler struct {
	store       sessions.Store
	fulfillSvc  service.FulfillmentService
	fulfillRepo repository.FulfillmentRepository
	tierRepo    repository.TierRepository
	customerRepo repository.CustomerRepository
	timelineRepo repository.TimelineRepository
	shippingRepo repository.ShippingAddressRepository
	exRepo      repository.ExceptionRepository
	encSvc      service.EncryptionService
}

func NewPageFulfillmentHandler(
	store sessions.Store,
	fulfillSvc service.FulfillmentService,
	fulfillRepo repository.FulfillmentRepository,
	tierRepo repository.TierRepository,
	customerRepo repository.CustomerRepository,
	timelineRepo repository.TimelineRepository,
	shippingRepo repository.ShippingAddressRepository,
	exRepo repository.ExceptionRepository,
	encSvc service.EncryptionService,
) *PageFulfillmentHandler {
	return &PageFulfillmentHandler{
		store: store, fulfillSvc: fulfillSvc, fulfillRepo: fulfillRepo,
		tierRepo: tierRepo, customerRepo: customerRepo, timelineRepo: timelineRepo,
		shippingRepo: shippingRepo, exRepo: exRepo, encSvc: encSvc,
	}
}

func (h *PageFulfillmentHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	filters := repository.FulfillmentFilters{}
	if s := c.Query("status"); s != "" { filters.Status = domain.FulfillmentStatus(s) }
	if t := c.Query("tier_id"); t != "" { id, _ := uuid.Parse(t); filters.TierID = &id }
	if cust := c.Query("customer_id"); cust != "" { id, _ := uuid.Parse(cust); filters.CustomerID = &id }
	if t := c.Query("type"); t != "" { filters.Type = domain.FulfillmentType(t) }
	if df := c.Query("date_from"); df != "" {
		if t, err := time.Parse("2006-01-02", df); err == nil { filters.DateFrom = &t }
	}
	if dt := c.Query("date_to"); dt != "" {
		if t, err := time.Parse("2006-01-02", dt); err == nil { filters.DateTo = &t }
	}

	page := queryInt(c, "page", 1)
	const size = 20
	fulfillments, total, _ := h.fulfillRepo.List(ctx, filters, domain.PageRequest{Page: page, PageSize: size})
	tiers, _ := h.tierRepo.List(ctx, "", false)
	customers, _, _ := h.customerRepo.List(ctx, "", domain.PageRequest{Page: 1, PageSize: 500}, false)

	// Build name lookup maps
	tierNames := map[uuid.UUID]string{}
	for _, t := range tiers { tierNames[t.ID] = t.Name }
	custNames := map[uuid.UUID]string{}
	for _, cu := range customers { custNames[cu.ID] = cu.Name }

	items := make([]fview.ListItem, len(fulfillments))
	for i, f := range fulfillments {
		items[i] = fview.ListItem{
			Fulfillment:  f,
			TierName:     tierNames[f.TierID],
			CustomerName: custNames[f.CustomerID],
		}
	}

	renderPage(c, http.StatusOK, fview.List(pageCtx(c, h.store), fview.ListData{
		Items:     items,
		Pager:     view.NewPagination(page, size, total, "/fulfillments", ""),
		Filters:   fview.ListFilters{Status: string(filters.Status), Type: string(filters.Type)},
		Tiers:     tiers,
		Customers: customers,
		CanCreate: canEdit(c, h.store),
	}))
}

func (h *PageFulfillmentHandler) ShowDetail(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	f, err := h.fulfillRepo.GetByID(ctx, id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	// Tier and customer names
	tierName := f.TierID.String()
	if t, err2 := h.tierRepo.GetByID(ctx, f.TierID); err2 == nil { tierName = t.Name }
	custName := f.CustomerID.String()
	if cu, err2 := h.customerRepo.GetByID(ctx, f.CustomerID); err2 == nil { custName = cu.Name }

	timeline, _ := h.timelineRepo.ListByFulfillmentID(ctx, id)
	shippingAddr, _ := h.shippingRepo.GetByFulfillmentID(ctx, id)
	exceptions, _ := h.exRepo.List(ctx, repository.ExceptionFilters{FulfillmentID: &id})

	// Build timeline events
	tevents := make([]fview.TimelineEvent, len(timeline))
	for i, ev := range timeline { tevents[i] = fview.TimelineEvent{TimelineEvent: ev} }

	// Decrypt and mask voucher
	voucherMasked := ""
	if f.VoucherCodeEncrypted != nil {
		if plain, err2 := h.encSvc.DecryptToString(f.VoucherCodeEncrypted); err2 == nil {
			voucherMasked = util.MaskVoucherCode(plain)
		}
	}

	exItems := make([]fview.ExceptionItem, len(exceptions))
	for i, ex := range exceptions { exItems[i] = fview.ExceptionItem{FulfillmentException: ex} }

	// Decrypt shipping address for display
	var shippingAddrResp *domain.ShippingAddressResponse
	if shippingAddr != nil {
		line1, _ := h.encSvc.DecryptToString(shippingAddr.Line1Encrypted)
		line2, _ := h.encSvc.DecryptToString(shippingAddr.Line2Encrypted)
		shippingAddrResp = &domain.ShippingAddressResponse{
			Line1:   line1,
			Line2:   line2,
			City:    shippingAddr.City,
			State:   shippingAddr.State,
			ZipCode: shippingAddr.ZipCode,
		}
	}

	renderPage(c, http.StatusOK, fview.Detail(pageCtx(c, h.store), fview.DetailData{
		Fulfillment:   *f,
		TierName:      tierName,
		CustomerName:  custName,
		Timeline:      tevents,
		Exceptions:    exItems,
		ShippingAddr:  shippingAddrResp,
		VoucherMasked: voucherMasked,
		CanTransition: canEdit(c, h.store),
	}))
}

func (h *PageFulfillmentHandler) ShowCreate(c *gin.Context) {
	ctx := c.Request.Context()
	tiers, _ := h.tierRepo.List(ctx, "", false)
	customers, _, _ := h.customerRepo.List(ctx, "", domain.PageRequest{Page: 1, PageSize: 500}, false)
	renderPage(c, http.StatusOK, fview.CreateForm(pageCtx(c, h.store), fview.CreateFormData{
		Tiers: tiers, Customers: customers,
	}))
}

func (h *PageFulfillmentHandler) PostCreate(c *gin.Context) {
	ctx := c.Request.Context()
	sess, _ := h.store.Get(c.Request, "fulfillops")
	actorID, _ := uuid.Parse(sess.Values["userID"].(string))

	tierID, _ := uuid.Parse(c.PostForm("tier_id"))
	customerID, _ := uuid.Parse(c.PostForm("customer_id"))
	fType := domain.FulfillmentType(c.PostForm("type"))

	ctx = service.WithUserID(ctx, actorID)
	f, err := h.fulfillSvc.Create(ctx, service.CreateFulfillmentInput{
		TierID: tierID, CustomerID: customerID, Type: fType,
	})
	if err != nil {
		redirectWithFlash(c, h.store, "/fulfillments/new", "error", err.Error())
		return
	}
	redirectWithFlash(c, h.store, "/fulfillments/"+f.ID.String(), "success", "Fulfillment created.")
}

func (h *PageFulfillmentHandler) PostTransition(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	sess, _ := h.store.Get(c.Request, "fulfillops")
	actorID, _ := uuid.Parse(sess.Values["userID"].(string))

	toStatus := domain.FulfillmentStatus(c.PostForm("to_status"))
	input := service.TransitionInput{
		FulfillmentID: id,
		ToStatus:      toStatus,
	}
	if cn := c.PostForm("carrier_name"); cn != "" { input.CarrierName = &cn }
	if tn := c.PostForm("tracking_number"); tn != "" { input.TrackingNumber = &tn }
	if vc := c.PostForm("voucher_code"); vc != "" { input.VoucherCode = []byte(vc) }
	if ve := c.PostForm("voucher_expiration"); ve != "" {
		if t, err := time.Parse("2006-01-02", ve); err == nil { input.VoucherExpiration = &t }
	}
	if r := c.PostForm("reason"); r != "" { input.Reason = &r }

	ctx = service.WithUserID(ctx, actorID)
	if _, err := h.fulfillSvc.Transition(ctx, input); err != nil {
		redirectWithFlash(c, h.store, "/fulfillments/"+id.String(), "error", err.Error())
		return
	}

	// Create shipping address for physical READY_TO_SHIP transitions
	if toStatus == domain.StatusReadyToShip {
		if line1 := c.PostForm("addr_line1"); line1 != "" {
			line1Enc, _ := h.encSvc.Encrypt([]byte(line1))
			line2Enc, _ := h.encSvc.Encrypt([]byte(c.PostForm("addr_line2")))
			addr := &domain.ShippingAddress{
				FulfillmentID: id,
				Line1Encrypted: line1Enc,
				Line2Encrypted: line2Enc,
				City:    c.PostForm("addr_city"),
				State:   c.PostForm("addr_state"),
				ZipCode: c.PostForm("addr_zip"),
			}
			_, _ = h.shippingRepo.CreateNoTx(ctx, addr)
		}
	}

	redirectWithFlash(c, h.store, "/fulfillments/"+id.String(), "success", "Status updated.")
}

func (h *PageFulfillmentHandler) PostDelete(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	sess, _ := h.store.Get(c.Request, "fulfillops")
	deletedBy, _ := uuid.Parse(sess.Values["userID"].(string))
	_ = h.fulfillRepo.SoftDelete(ctx, id, deletedBy)
	redirectWithFlash(c, h.store, "/fulfillments", "success", "Fulfillment deleted.")
}

func (h *PageFulfillmentHandler) PostRestore(c *gin.Context) {
	ctx := c.Request.Context()
	id, _ := uuid.Parse(c.Param("id"))
	if err := h.fulfillRepo.Restore(ctx, id); err != nil {
		redirectWithFlash(c, h.store, "/admin/recovery", "error", "Restore failed.")
		return
	}
	redirectWithFlash(c, h.store, "/admin/recovery", "success", "Fulfillment restored.")
}
