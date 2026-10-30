# Web routes

The feature catalog in `internal/features/catalog.go` is the source of truth for the navigation and route metadata. This page groups the main user-facing areas; contextual routes may be added by detail views and dialogs.

| Area | Route |
|---|---|
| Dashboard | `/` |
| Repositories | `/repositories` |
| Issues | `/issues` |
| Projects | `/projects` |
| Pull Requests | `/pull-requests` |
| Knowledge Graph | `/graph` |
| Concepts | `/concepts` |
| Shelf | `/shelf` |
| Notes | `/notes` |
| Quotes | `/quotes` |
| Activity | `/activity` |
| Daily Plans | `/plans` |
| Labels and Milestones | `/planning` |
| Scheduled Jobs | `/jobs` |
| Actions Center | `/commands` |
| Profile | `/profile` |
| Remove Items | `/remove` |
| Logs | `/logs` |
| Help | `/help` |
| Notifications | `/notifications` |
| Settings | `/settings` |
| AI Settings | `/settings/ai` |
| Notification Settings | `/settings/notifications` |

Repository, Issue, Pull Request, Project, Concept, and Book pages have contextual detail routes. Shelf book routes include `/shelf/:slug` and `/shelf/:slug/read`.
