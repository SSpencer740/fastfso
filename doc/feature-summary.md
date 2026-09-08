# Feature Summary

Summary of all feature specs in `features/`, along with what's already been built and a recommended build order.

## What's Already Built

- **Authentication** — login, passkeys, TOTP, email codes, SSO (SAML/OIDC)
- **User Roles** — administrator, fso, read_only_fso, individual_contributor (DB + middleware enforcement)
- **Multi-tenant + Sub-orgs** — tables, associations, user-suborganization join table
- **fastFSO Internal Admin Panel** — tenant CRUD, identity management, SSO config, session/audit views. Missing: AI feature toggles, bulk user upload
- **User Settings** — password change, passkey management, TOTP management. Missing: email notification frequency
- **Session management, audit logging, telemetry** — all solid
- **Email service** — exists but only for auth codes and admin invites

## Feature Specs Summary

### 1. User Roles & Permissions (`userRoleDesc.md`)

Four roles with scoped access:

| Role | Access |
|------|--------|
| Administrator | Full read/write across the tenancy |
| FSO | Read/write for users in their assigned sub-org(s) |
| Read Only FSO | View-only for their assigned sub-org(s) |
| Individual Contributor | Can only see/edit their own records |

Each user also has an **internal position** (individual contributor, management, contractor) which controls visibility of certain reporting sections (e.g., FCL reporting is management-only).

### 2. fastFSO Internal Admin Panel (`fastfsoInternalAdminPanel.md`)

fastFSO-the-company's admin panel for managing customers:

- Create/manage user accounts per customer with total count
- Assign roles and sub-organizations (users can belong to multiple sub-orgs)
- Reset passwords
- Toggle AI features per customer (chatbot, MCP) with RBAC
- Nice-to-have: bulk user upload

**Status:** Mostly built. Missing AI feature toggles and bulk upload.

### 3. Customer Admin Dashboard (`customerAdminDashboard.md`)

Customer-facing admin panel (admin-only visibility):

- Assign users to sub-orgs
- Assign FSOs to sub-orgs
- Reassign user roles
- Search/view all team members

**Status:** Not built.

### 4. Task Dashboard (`maintaskDashboard.md`)

Visual overview of task completion with charts/graphs:

- **All users:** pie chart/graph of personal outstanding tasks within a custom date range, plus a sorted list of upcoming due dates (closest first)
- **Non-IC roles:** same visualization for everyone in their sub-org(s), filterable by sub-org
- **ICs:** see their assigned FSO's contact info at the bottom

**Status:** Not built.

### 5. Task Panel (`taskPanel.md`)

The core task management system:

- All users see a filterable table of their tasks (filter by completion status, date range, keyword search)
- Clicking a task opens a detail popup with description, web links, due date, text input, file upload
- **LLM validation:** when an assignee marks a task complete, an LLM checks uploaded docs match the task requirements (only when the assigner marks documentation as expected)
- Completing a task auto-creates an approval task for the assigner
- Subtask support
- **Non-IC roles:** view all tasks in their purview, filter by sub-org, keyword search
- **FSOs/Admins:** can create tasks and bulk-assign (e.g., "annual training" to 50 users in a sub-org)

**Status:** Not built. No DB tables, no endpoints, no UI.

### 6. Report Foreign Travel (`reportForeignTravel.md`)

Multi-step travel reporting wizard:

- Table of reported travel (trip name, dates, upcoming/completed status) with filter/search
- "Report Travel" button opens a popup form
- Multi-country trip support: user declares countries, gets per-country sub-forms with arrow navigation
- File upload for itineraries
- Nice-to-have: LLM parses uploaded itinerary docs to auto-populate fields
- Submitting creates a task for the assigned FSO
- **Post-trip debrief:** auto-email with debrief questions after return date
- **Non-IC panel:** view all travel in purview, filter by sub-org, approve/deny after DISS submission

Input fields: name, email, passport number, file upload, trip dates, reason for travel (radio: NGO, official, vacation, other), transportation mode (checkboxes: air, auto, sea, train, walking), foreign companions (yes/no + details), planned foreign contacts (yes/no + details), emergency contact (US-based, not traveling with), additional comments.

**Status:** Not built.

### 7. Report Other / Life Events (`reportOther.md`)

The most content-heavy feature. Covers mandatory SEAD-3 reporting:

- Table of reports (reporter name, report type, created date, status) with filter/search
- "Report life event for myself or others" button opens a form
- First choice: self-reporting vs. reporting on behalf of someone else
- **10 self-report categories:** Alcohol/Drug Treatment, Arrests, Cohabitant, Elicitation/Blackmail, Financial, Foreign Activities, Foreign Contacts, Marriage/Divorce, Media Contact, Other — each with specific prompts
- **Management-only section:** FCL reporting (change of ownership, address change, inability to safeguard classified info)
- Status workflow: unreviewed → under review → processed
- Submitting creates a task for the assigned FSO with title "Process [TYPE] for [NAME]"
- **Non-IC panel:** view all reports in purview, filter by sub-org, update status dropdown

**Status:** Not built.

### 8. Visit Requests (`visitRequests.md`)

Facility visit request submission and processing:

- Table of submitted visit requests
- "Submit a Visit Request" form
- **Clone from previous request** — users frequently revisit the same locations, pre-populates form
- CUI/classified info warning on the description field
- **Non-IC panel:** view all requests in purview, filter by sub-org, mark as processed/pending after DISS submission

Input fields: name, email, visit dates, DISS SMO code, company/org name, visit address, POC (name/email/phone), high-level description (with CUI warning), access required (Confidential/Secret/TS/TS-SCI), security POC (name/email/phone).

**Status:** Not built.

### 9. Wiki / Government Security Homepage (`wikiPage.md`)

Internal knowledge base:

- Page title: "Government Security Homepage"
- FSOs/Admins can post to the whole org or specific sub-orgs
- Posts displayed as tiles, clickable for full popup view
- "Your FSO" tile showing the user's assigned FSO and email

**Status:** Not built.

### 10. Chatbot — fsoBOT (`chatbot.md`)

AI-powered floating chatbot accessible from anywhere in the app:

- Answers FSO-domain questions
- Can query user data scoped to the user's role (IC sees only their data, FSO sees their sub-org's data)
- Always-visible button on screen

**Status:** Not built.

### 11. Reminder Emails (`reminderEmails.md`)

Automated email notifications:

- New task notification (frequency based on user settings)
- Due date reminders at 30 days, 7 days, and 24 hours before deadline

**Status:** Not built. Email service exists but only handles auth codes and invites.

### 12. User Settings (`userSettings.md`)

Personal settings page:

- Reset password (already built)
- Email notification frequency: every task immediately, or daily digest (not built)

**Status:** Partially built.

## Recommended Build Order

The task system is the backbone — travel reports, life event reports, and visit requests all generate tasks for FSOs. Without it, no reporting workflow has anywhere to land.

| Priority | Feature | Rationale |
|----------|---------|-----------|
| 1 | **Task System** (Task Panel) | Unblocks all reporting workflows. Every report creates a task for the FSO. |
| 2 | **Customer Admin Dashboard** | Lightweight. Needed for customer self-service (assign users to sub-orgs, reassign roles). |
| 3 | **Foreign Travel** or **Report Other** | Core compliance workflows. These are what FSOs and ICs use daily. |
| 4 | **Visit Requests** | Another common compliance workflow, similar patterns to reporting. |
| 5 | **Wiki Page** | Relatively simple. Gives FSOs a place to share info with their team. |
| 6 | **Task Dashboard** (charts) | Visualization layer on top of the task system. Needs tasks to exist first. |
| 7 | **Reminder Emails** + user settings for frequency | Depends on tasks existing. Needs email service expansion + cron jobs. |
| 8 | **Chatbot (fsoBOT)** | Most complex. Needs data in the system to query. AI integration, role-scoped data access. |

### Foundation work before feature #1

Before building the task system, the frontend needs an app shell:

- **Sidebar navigation + nested routing** — the container all feature pages live in
- **Role-aware nav rendering** — show/hide items based on user role
- **Enhanced shared components** — filterable data tables, form modals, status badges, sub-org filter (reused by nearly every feature)
