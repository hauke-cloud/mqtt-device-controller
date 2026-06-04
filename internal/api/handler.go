package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	iov1 "github.com/hauke-cloud/mqtt-device-controller/api/v1alpha1"
	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Handler holds dependencies for all REST handlers.
type Handler struct {
	k8s       client.Client
	namespace string
}

// NewHandler constructs a Handler.
func NewHandler(k8s client.Client, namespace string) *Handler {
	return &Handler{k8s: k8s, namespace: namespace}
}

// DeviceView is the REST representation of an MQTTDevice.
type DeviceView struct {
	Name         string            `json:"name"`
	FriendlyName string            `json:"friendlyName"`
	IEEEAddr     string            `json:"ieeeAddr"`
	ShortAddr    string            `json:"shortAddr,omitempty"`
	Bridge       string            `json:"bridge"`
	Disabled     bool              `json:"disabled"`
	ModelID      string            `json:"modelId,omitempty"`
	Manufacturer string            `json:"manufacturer,omitempty"`
	Reachable    *bool             `json:"reachable,omitempty"`
	BatteryPct   *int32            `json:"batteryPercentage,omitempty"`
	LinkQuality  *int32            `json:"linkQuality,omitempty"`
	LastSeenTime string            `json:"lastSeenTime,omitempty"`
}

func toView(d iov1.MQTTDevice) DeviceView {
	v := DeviceView{
		Name:         d.Name,
		FriendlyName: d.Spec.FriendlyName,
		IEEEAddr:     d.Spec.IEEEAddr,
		ShortAddr:    d.Spec.ShortAddr,
		Bridge:       d.Spec.BridgeRef.Name,
		Disabled:     d.Spec.Disabled,
		ModelID:      d.Status.ModelID,
		Manufacturer: d.Status.Manufacturer,
		Reachable:    d.Status.Reachable,
		BatteryPct:   d.Status.BatteryPct,
		LinkQuality:  d.Status.LinkQuality,
	}
	if d.Status.LastSeenTime != nil {
		v.LastSeenTime = d.Status.LastSeenTime.UTC().Format("2006-01-02T15:04:05.999999999Z")
	}
	return v
}

// ListDevices handles GET /api/v1/devices
func (h *Handler) ListDevices(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := parsePagination(r)
	if err != nil {
		writeError(w, fmt.Errorf("%w: %v", ErrInvalidInput, err))
		return
	}

	var list iov1.MQTTDeviceList
	if err := h.k8s.List(r.Context(), &list, client.InNamespace(h.namespace)); err != nil {
		writeError(w, err)
		return
	}

	all := list.Items
	total := len(all)
	if offset >= total {
		writeCollection(w, []DeviceView{}, total, limit, offset)
		return
	}
	end := offset + limit
	if end > total {
		end = total
	}

	views := make([]DeviceView, 0, end-offset)
	for _, d := range all[offset:end] {
		views = append(views, toView(d))
	}
	writeCollection(w, views, total, limit, offset)
}

// GetDevice handles GET /api/v1/devices/{identifier}
// identifier can be CR name, friendlyName, short addr (0x…), or long IEEE addr.
func (h *Handler) GetDevice(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "identifier")

	// Try direct CR name lookup first (fast path).
	var dev iov1.MQTTDevice
	err := h.k8s.Get(r.Context(), types.NamespacedName{Name: id, Namespace: h.namespace}, &dev)
	if err == nil {
		writeJSON(w, http.StatusOK, toView(dev))
		return
	}

	// Fall back to scanning all devices.
	var list iov1.MQTTDeviceList
	if listErr := h.k8s.List(r.Context(), &list, client.InNamespace(h.namespace)); listErr != nil {
		writeError(w, listErr)
		return
	}

	idLower := strings.ToLower(id)
	for _, d := range list.Items {
		if strings.ToLower(d.Spec.FriendlyName) == idLower ||
			strings.ToLower(d.Spec.IEEEAddr) == idLower ||
			strings.ToLower(d.Spec.ShortAddr) == idLower {
			writeJSON(w, http.StatusOK, toView(d))
			return
		}
	}

	writeError(w, fmt.Errorf("device %q: %w", id, ErrNotFound))
}

func parsePagination(r *http.Request) (limit, offset int, err error) {
	limit = 20
	offset = 0

	if v := r.URL.Query().Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil || limit < 1 || limit > 100 {
			return 0, 0, fmt.Errorf("limit must be 1–100")
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		offset, err = strconv.Atoi(v)
		if err != nil || offset < 0 {
			return 0, 0, fmt.Errorf("offset must be >= 0")
		}
	}
	return limit, offset, nil
}
