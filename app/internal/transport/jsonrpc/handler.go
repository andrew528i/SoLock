package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/solock/solock/internal/application"
	"github.com/solock/solock/internal/domain"
	"github.com/solock/solock/internal/usecase"
)

type Handler struct {
	app       *application.App
	token     string
	shutdown  chan struct{}
}

func NewHandler(app *application.App, token string, shutdown chan struct{}) *Handler {
	return &Handler{app: app, token: token, shutdown: shutdown}
}

func (h *Handler) Handle(req *Request) *Response {
	if req.Method != "unlock" {
		if h.app.IsLocked() {
			if h.app.IsExpired() {
				h.app.Lock()
			}
			if h.app.IsLocked() && req.Method != "status" && req.Method != "shutdown" {
				return errorResponse(req.ID, ErrCodeLocked, "vault is locked")
			}
		}
	}

	switch req.Method {
	case "unlock":
		return h.handleUnlock(req)
	case "lock":
		return h.handleLock(req)
	case "status":
		return h.handleStatus(req)
	case "list_entries":
		return h.handleListEntries(req)
	case "get_entry":
		return h.handleGetEntry(req)
	case "search_entries":
		return h.handleSearchEntries(req)
	case "add_entry":
		return h.handleAddEntry(req)
	case "update_entry":
		return h.handleUpdateEntry(req)
	case "delete_entry":
		return h.handleDeleteEntry(req)
	case "get_dashboard":
		return h.handleGetDashboard(req)
	case "sync":
		return h.handleSync(req)
	case "deploy_program":
		return h.handleDeployProgram(req)
	case "initialize_vault":
		return h.handleInitVault(req)
	case "reset_vault":
		return h.handleResetVault(req)
	case "clear_local_data":
		return h.handleClearLocalData(req)
	case "generate_password":
		return h.handleGeneratePassword(req)
	case "generate_totp":
		return h.handleGenerateTOTP(req)
	case "get_config":
		return h.handleGetConfig(req)
	case "set_config":
		return h.handleSetConfig(req)
	case "list_groups":
		return h.handleListGroups(req)
	case "add_group":
		return h.handleAddGroup(req)
	case "update_group":
		return h.handleUpdateGroup(req)
	case "delete_group":
		return h.handleDeleteGroup(req)
	case "purge_group":
		return h.handlePurgeGroup(req)
	case "shutdown":
		return h.handleShutdown(req)
	default:
		return errorResponse(req.ID, ErrCodeNotFound, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (h *Handler) handleUnlock(req *Request) *Response {
	var params struct {
		Password       string `json:"password"`
		TimeoutMinutes int    `json:"timeout_minutes"`
		RPCURL         string `json:"rpc_url"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}
	if params.Password == "" {
		return errorResponse(req.ID, ErrCodeInvalidReq, "password is required")
	}
	if params.RPCURL == "" {
		params.RPCURL = "https://api.devnet.solana.com"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.app.OnUnlockWithTimeout(ctx, params.Password, params.RPCURL, params.TimeoutMinutes); err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}

	return successResponse(req.ID, map[string]any{
		"ok":         true,
		"expires_at": h.app.ExpiresAt().Unix(),
	})
}

func (h *Handler) handleLock(req *Request) *Response {
	h.app.Lock()
	return successResponse(req.ID, map[string]bool{"ok": true})
}

func (h *Handler) handleStatus(req *Request) *Response {
	locked := h.app.IsLocked()
	result := map[string]any{
		"locked": locked,
	}
	if !locked {
		exp := h.app.ExpiresAt()
		if !exp.IsZero() {
			result["expires_at"] = exp.Unix()
			result["remaining_seconds"] = int(time.Until(exp).Seconds())
		}
	}
	return successResponse(req.ID, result)
}

func (h *Handler) handleListEntries(req *Request) *Response {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entries, err := h.app.ListEntries.Execute(ctx)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, entriesToJSON(entries))
}

func (h *Handler) handleGetEntry(req *Request) *Response {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	entry, err := h.app.GetEntry.Execute(ctx, params.ID)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, entryToJSON(entry))
}

func (h *Handler) handleSearchEntries(req *Request) *Response {
	var params struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := h.app.SearchEntries.Execute(ctx, params.Query)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, entriesToJSON(result.Entries))
}

func (h *Handler) handleAddEntry(req *Request) *Response {
	var params struct {
		Type       string            `json:"type"`
		Name       string            `json:"name"`
		Fields     map[string]string `json:"fields"`
		GroupIndex *uint32           `json:"group_index,omitempty"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	entry, err := domain.NewEntry(id, domain.EntryType(params.Type), params.Name, params.Fields)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInvalidReq, err.Error())
	}
	if params.GroupIndex != nil {
		entry.SetGroupIndex(params.GroupIndex)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := h.app.AddEntry.Execute(ctx, entry)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]any{
		"id":       id,
		"on_chain": result.OnChain,
	})
}

func (h *Handler) handleUpdateEntry(req *Request) *Response {
	var params struct {
		ID         string            `json:"id"`
		Name       string            `json:"name"`
		Fields     map[string]string `json:"fields"`
		GroupIndex *uint32           `json:"group_index"`
		ClearGroup bool              `json:"clear_group"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	entry, err := h.app.GetEntry.Execute(ctx, params.ID)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}

	if params.Name != "" {
		entry.SetName(params.Name)
	}
	if params.Fields != nil {
		entry.SetFields(params.Fields)
	}
	if params.ClearGroup {
		entry.SetGroupIndex(nil)
	} else if params.GroupIndex != nil {
		entry.SetGroupIndex(params.GroupIndex)
	}

	result, err := h.app.UpdateEntry.Execute(ctx, entry)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]any{
		"ok":       true,
		"on_chain": result.OnChain,
	})
}

func (h *Handler) handleDeleteEntry(req *Request) *Response {
	var params struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	entry, err := h.app.GetEntry.Execute(ctx, params.ID)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}

	result, err := h.app.DeleteEntry.Execute(ctx, entry)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]any{
		"ok":       true,
		"on_chain": result.OnChain,
	})
}

func (h *Handler) handleGetDashboard(req *Request) *Response {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	info, err := h.app.GetDashboard.Execute(ctx)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}

	return successResponse(req.ID, map[string]any{
		"deployer_address": info.DeployerAddress,
		"program_id":       info.ProgramID,
		"balance":          info.Balance,
		"program_deployed": info.ProgramDeployed,
		"vault_exists":     info.VaultExists,
		"entry_count":      info.EntryCount,
		"password_count":   info.PasswordCount,
		"note_count":       info.NoteCount,
		"card_count":       info.CardCount,
		"totp_count":       info.TOTPCount,
		"group_count":      info.GroupCount,
		"last_sync_at":     info.LastSyncAt.Unix(),
		"network":          info.Network,
		"rpc_url":          info.RPCURL,
	})
}

func (h *Handler) handleSync(req *Request) *Response {
	if h.app.Sync == nil {
		return errorResponse(req.ID, ErrCodeInternal, "not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	err := h.app.Sync.Execute(ctx, func(p usecase.SyncProgress) {})
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]bool{"ok": true})
}

func (h *Handler) handleDeployProgram(req *Request) *Response {
	if h.app.DeployProgram == nil {
		return errorResponse(req.ID, ErrCodeInternal, "not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := h.app.DeployProgram.Execute(ctx); err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]bool{"ok": true})
}

func (h *Handler) handleInitVault(req *Request) *Response {
	if h.app.InitVault == nil {
		return errorResponse(req.ID, ErrCodeInternal, "not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.app.InitVault.Execute(ctx); err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]bool{"ok": true})
}

func (h *Handler) handleResetVault(req *Request) *Response {
	if h.app.ResetVault == nil {
		return errorResponse(req.ID, ErrCodeInternal, "not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.app.ResetVault.Execute(ctx); err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]bool{"ok": true})
}

func (h *Handler) handleClearLocalData(req *Request) *Response {
	if h.app.ClearLocalData == nil {
		return errorResponse(req.ID, ErrCodeInternal, "not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.app.ClearLocalData.Execute(ctx); err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]bool{"ok": true})
}

func (h *Handler) handleGeneratePassword(req *Request) *Response {
	var params domain.PasswordGenConfig
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}
	if params.Length == 0 {
		params = *domain.DefaultPasswordGenConfig()
	}
	password := h.app.GeneratePassword.Execute(&params)
	return successResponse(req.ID, map[string]string{"password": password})
}

func (h *Handler) handleGenerateTOTP(req *Request) *Response {
	var params struct {
		Secret string `json:"secret"`
		Digits int    `json:"digits"`
		Period int    `json:"period"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}

	result, err := h.app.GenerateTOTP.Execute(params.Secret, params.Digits, params.Period)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]any{
		"code":      result.Code,
		"remaining": result.Remaining,
	})
}

func (h *Handler) handleGetConfig(req *Request) *Response {
	var params struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	value, err := h.app.GetConfig.Execute(ctx, params.Key)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]string{"value": value})
}

func (h *Handler) handleSetConfig(req *Request) *Response {
	var params struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.app.SetConfig.Execute(ctx, params.Key, params.Value); err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]bool{"ok": true})
}

func (h *Handler) handleShutdown(req *Request) *Response {
	h.app.Lock()
	close(h.shutdown)
	return successResponse(req.ID, map[string]bool{"ok": true})
}

func (h *Handler) handleListGroups(req *Request) *Response {
	if h.app.ListGroups == nil {
		return errorResponse(req.ID, ErrCodeInternal, "not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	groups, err := h.app.ListGroups.Execute(ctx)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, groupsToJSON(groups))
}

func (h *Handler) handleAddGroup(req *Request) *Response {
	var params struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}
	if h.app.AddGroup == nil {
		return errorResponse(req.ID, ErrCodeInternal, "not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, err := h.app.AddGroup.Execute(ctx, params.Name, domain.GroupColor(params.Color))
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]any{
		"index":    result.Group.Index(),
		"on_chain": result.OnChain,
	})
}

func (h *Handler) handleUpdateGroup(req *Request) *Response {
	var params struct {
		Index uint32  `json:"index"`
		Name  string  `json:"name"`
		Color *string `json:"color"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}
	if h.app.UpdateGroup == nil {
		return errorResponse(req.ID, ErrCodeInternal, "not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	target, err := h.app.Groups().Get(ctx, params.Index)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	if target == nil {
		return errorResponse(req.ID, ErrCodeInternal, "group not found")
	}

	target.SetName(params.Name)
	if params.Color != nil {
		c := domain.GroupColor(*params.Color)
		if err := domain.ValidateGroupColor(c); err != nil {
			return errorResponse(req.ID, ErrCodeInvalidReq, err.Error())
		}
		target.SetColor(c)
	}
	result, err := h.app.UpdateGroup.Execute(ctx, target)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]any{
		"ok":       true,
		"on_chain": result.OnChain,
	})
}

func (h *Handler) handleDeleteGroup(req *Request) *Response {
	var params struct {
		Index         uint32 `json:"index"`
		DeleteEntries bool   `json:"delete_entries"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}
	if h.app.DeleteGroup == nil {
		return errorResponse(req.ID, ErrCodeInternal, "not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := h.app.DeleteGroup.Execute(ctx, params.Index, params.DeleteEntries)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]any{
		"ok":       true,
		"on_chain": result.OnChain,
	})
}

func (h *Handler) handlePurgeGroup(req *Request) *Response {
	var params struct {
		Index uint32 `json:"index"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, ErrCodeParse, "invalid params")
	}
	if h.app.PurgeGroup == nil {
		return errorResponse(req.ID, ErrCodeInternal, "not connected")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := h.app.PurgeGroup.Execute(ctx, params.Index)
	if err != nil {
		return errorResponse(req.ID, ErrCodeInternal, err.Error())
	}
	return successResponse(req.ID, map[string]any{
		"ok":       true,
		"on_chain": result.OnChain,
	})
}

type entryJSON struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Name       string            `json:"name"`
	Fields     map[string]string `json:"fields"`
	HasTOTP    bool              `json:"has_totp"`
	SlotIndex  uint32            `json:"slot_index"`
	GroupIndex *uint32           `json:"group_index,omitempty"`
	CreatedAt  int64             `json:"created_at"`
	UpdatedAt  int64             `json:"updated_at"`
	AccessedAt int64             `json:"accessed_at"`
}

func entryToJSON(e *domain.Entry) *entryJSON {
	var accessedAt int64
	if !e.AccessedAt().IsZero() {
		accessedAt = e.AccessedAt().Unix()
	}
	return &entryJSON{
		ID:         e.ID(),
		Type:       string(e.Type()),
		Name:       e.Name(),
		Fields:     e.Fields(),
		HasTOTP:    e.HasTOTP(),
		SlotIndex:  e.SlotIndex(),
		GroupIndex: e.GroupIndex(),
		CreatedAt:  e.CreatedAt().Unix(),
		UpdatedAt:  e.UpdatedAt().Unix(),
		AccessedAt: accessedAt,
	}
}

func entriesToJSON(entries []*domain.Entry) []*entryJSON {
	result := make([]*entryJSON, 0, len(entries))
	for _, e := range entries {
		result = append(result, entryToJSON(e))
	}
	return result
}

type groupJSON2 struct {
	Index     uint32           `json:"index"`
	Name      string           `json:"name"`
	Color     domain.GroupColor `json:"color,omitempty"`
	Deleted   bool             `json:"deleted"`
	CreatedAt int64            `json:"created_at"`
	UpdatedAt int64            `json:"updated_at"`
}

func groupToJSON(g *domain.Group) *groupJSON2 {
	return &groupJSON2{
		Index:     g.Index(),
		Name:      g.Name(),
		Color:     g.Color(),
		Deleted:   g.Deleted(),
		CreatedAt: g.CreatedAt().Unix(),
		UpdatedAt: g.UpdatedAt().Unix(),
	}
}

func groupsToJSON(groups []*domain.Group) []*groupJSON2 {
	result := make([]*groupJSON2, 0, len(groups))
	for _, g := range groups {
		result = append(result, groupToJSON(g))
	}
	return result
}
