# Design Document: Chatbot AI Contextual Category Creation

**Date:** 2026-08-25
**Status:** Approved

## 1. Objective
Allow the Nexo AI chatbot to automatically create new categories based on the conversation context when existing categories do not meet the user's need, as well as support an explicit tool for category creation.

---

## 2. Requirements & Behavior

### 2.1. Explicit Category Creation (`create_category` Tool)
- AI can call the tool `create_category` when the user explicitly requests to create a new category (e.g., *"Tạo danh mục Học tập", "Tạo cho tôi danh mục Tiền điện loại Chi tiêu"*).
- Tool Arguments:
  - `name` (string, required): Category name.
  - `type` (string, optional: `EXPENSE` | `INCOME` | `INVESTMENT`, default: `EXPENSE`).
  - `description` (string, optional): Description or notes.
- Deduplication: If a category with the same name (case-insensitive) already exists for this user, return the existing category instead of duplicating.
- Action Card: Returns `CATEGORY_CREATED` Action Card so the web chat UI renders a visual card with reload triggers.

### 2.2. Contextual Auto-Creation in `create_transaction`
- In `toolCreateTransaction`:
  - When the specified `categoryName` does not match any existing categories of the user:
    - Automatically create a new `model.Category` with the given `categoryName` and matching transaction type (`EXPENSE` or `INCOME`).
    - Link the transaction to this freshly created category.
    - Set the flag `createdCategory` in the response card data so the UI or AI message clearly indicates that a new category was created dynamically.

### 2.3. System Prompt Updates
- Update the system prompt in `internal/service/chat_service.go` to instruct AI:
  - When the user asks to create/add a new category -> call `create_category`.
  - When creating a transaction and the category does not exist in the default list -> specify the most descriptive category name in `create_transaction` and it will be created automatically.

### 2.4. Frontend Web (`nexo-web`) Integration
- **Action Card**:
  - Support `CATEGORY_CREATED` in `AIChatWidget.tsx` and `AIChatWidget.css` with a distinct theme color.
  - Dispatch event `categories-changed` when `CATEGORY_CREATED` or a transaction creating a new category is received.
- **Tools Catalog**:
  - Add `create_category` into `AI_TOOLS_CATALOG` under the `Giao dịch` category group with prompt examples.

---

## 3. Implementation Files

1. **Backend (`nexo-app-api`)**:
   - `internal/service/financial_tools.go`:
     - Add `create_category` to `GetFinancialToolDefinitions()`.
     - Implement `toolCreateCategory(ctx, userID, args)`.
     - Update `toolCreateTransaction` category fallback logic to create the exact requested category name instead of falling back to the first existing category.
     - Add `create_category` routing in `executeFinancialTool`.
   - `internal/service/chat_service.go`:
     - Update AI system prompt rules for category management.
   - `internal/service/financial_tools_test.go`:
     - Update tests to verify `create_category` definition.

2. **Frontend (`nexo-web`)**:
   - `src/components/chat/AIChatWidget.tsx`:
     - Register `create_category` in `AI_TOOLS_CATALOG`.
     - Handle `CATEGORY_CREATED` ActionCard rendering and `categories-changed` event dispatching.
   - `src/components/chat/AIChatWidget.css`:
     - Add style definition for `.chat-action-card.CATEGORY_CREATED`.

---

## 4. Verification Plan
- Run `go test ./...` in `nexo-app-api` to ensure all tests pass.
- Run `npm run build` in `nexo-web` to verify TypeScript compile & bundle without error.
