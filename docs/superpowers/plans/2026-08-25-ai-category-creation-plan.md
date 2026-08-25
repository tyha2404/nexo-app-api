# AI Chatbot Contextual Category Creation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable the Nexo AI chatbot to automatically create new categories based on conversational context during transaction creation, and provide an explicit `create_category` AI tool.

**Architecture:** Extend backend Go financial tools in `nexo-app-api` with a new `create_category` tool and enhance category resolution in `create_transaction` to dynamically create new categories when not matched. Update the system prompt to guide the AI. On the frontend (`nexo-web`), handle `CATEGORY_CREATED` ActionCard and register `create_category` in the AI tools catalog.

**Tech Stack:** Go 1.25, GORM, Chi, React 19, Vite, TypeScript, TailwindCSS.

---

### Task 1: Backend - Add `create_category` Tool and Dynamic Category Creation in `nexo-app-api`

**Files:**
- Modify: `internal/service/financial_tools.go`
- Modify: `internal/service/chat_service.go`
- Modify: `internal/service/financial_tools_test.go`

- [ ] **Step 1: Write/Update tests in `financial_tools_test.go`**
Add assertion for `create_category` in `TestGetFinancialToolDefinitions`.

- [ ] **Step 2: Run test to verify it fails**
Run: `cd /Users/tyha/Documents/nexo-project/nexo-app-api && go test ./internal/service/...`
Expected: FAIL asserting `create_category` exists in definitions.

- [ ] **Step 3: Implement `create_category` tool definition and execution in `financial_tools.go`**
1. Add `create_category` tool definition in `GetFinancialToolDefinitions()`.
2. Add `toolCreateCategory(ctx, userID, args)` method:
   - Extract `name` (required), `type` (optional: `EXPENSE`, `INCOME`, `INVESTMENT`), `description` (optional).
   - Check if category with matching name exists (case-insensitive). If found, return existing category.
   - If not found, create new `model.Category` via `s.categoryRepo.Create(ctx, ...)`.
   - Return `CATEGORY_CREATED` ActionCard with data.
3. Update `toolCreateTransaction`:
   - When `categoryName` is provided and doesn't match any existing category, create a new `model.Category` with that name and matching `txnTypeStr` instead of picking the first category.
4. Route `create_category` in `executeFinancialTool(ctx, userID, name, args)`.
5. Update System Prompt in `chat_service.go` to teach AI how to use `create_category` and dynamic category creation.

- [ ] **Step 4: Run tests to verify they pass**
Run: `cd /Users/tyha/Documents/nexo-project/nexo-app-api && go test ./...`
Expected: PASS

---

### Task 2: Frontend - Update Web UI with `CATEGORY_CREATED` Action Card and Catalog

**Files:**
- Modify: `nexo-web/src/components/chat/AIChatWidget.tsx`
- Modify: `nexo-web/src/components/chat/AIChatWidget.css`

- [ ] **Step 1: Update `AIChatWidget.tsx`**
1. Add `create_category` to `AI_TOOLS_CATALOG` under category `Giao dịch`.
2. In `handleSendMessage`, when receiving `event.actionCard` with `actionType === 'CATEGORY_CREATED'`, dispatch `window.dispatchEvent(new CustomEvent('categories-changed'))`.
3. In `renderActionCard`, add icon `🏷️ ` for `CATEGORY_CREATED`.

- [ ] **Step 2: Update `AIChatWidget.css`**
Add styles for `.chat-action-card.CATEGORY_CREATED` (e.g. `border-left-color: #8b5cf6;`).

- [ ] **Step 3: Run TypeScript check and build**
Run: `cd /Users/tyha/Documents/nexo-project/nexo-web && npm run build`
Expected: Build succeeds without TypeScript or bundling errors.

---

### Task 3: End-to-End Verification

- [ ] **Step 1: Run Go tests**
Run: `cd /Users/tyha/Documents/nexo-project/nexo-app-api && go test ./...`
Expected: All tests PASS.

- [ ] **Step 2: Run Web type-check and lint**
Run: `cd /Users/tyha/Documents/nexo-project/nexo-web && npm run build`
Expected: All checks PASS.
