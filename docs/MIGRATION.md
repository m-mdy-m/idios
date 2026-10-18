# Importing data from the previous Python IDIOS

The Go Engine uses a **separate database by default** (`~/.idios/idios-engine.sqlite3`). Do not point it at the previous Python database; the schemas differ and the new Engine must not mutate the original file.

## 1. Make a backup

Close the old IDIOS app, then back up its SQLite file. A common old path is `~/.idios/idios.db`; use the actual path from your installation if different. Keep the backup until validating the imported data.

## 2. Build the Go Engine

```sh
make build
```

## 3. Import into the new database

```sh
./dist/idios --db ~/.idios/idios-engine.sqlite3 legacy-import --from ~/.idios/idios.db
```

`--from` is opened read-only. The importer refuses the obvious case where source and target paths point to the same file.

## What V1 imports

- Concepts from the previous database.
- Tasks as Issues inside a new Repository named `Legacy Tasks`.
- Supported semantic relations between imported Concepts and Tasks, where endpoints can be mapped safely.
- Legacy metadata not represented by a first-class V1 field (for example goal/source IDs and old due/done dates) as migration context in the Issue description.

Legacy statuses are mapped as follows: active/incomplete Tasks become `open`; skipped Tasks become `cancelled`; done Tasks become `done` through an explicitly labelled synthetic migration PR and CompletionRecord. The synthetic PR makes the imported history satisfy the new invariant; it is clearly described as a migration record, not as an old PR or real Git commit.

A legacy Task does not always contain enough information to reconstruct a true workflow dependency. The importer does not guess blocking dependencies from semantic relation names. Review the resulting data and add explicit `issue depends --issue ... --on ...` edges where the workflow really requires a prerequisite.

## Not imported in this initial tool

The old Goals, Sources, Questions, Notes, schedules and time-tracking records are not yet mapped to first-class new entities. They remain untouched in the original database and its backup. Keep that file; the importer does not delete or rewrite it. A future migration can map those records once their Go-domain counterparts are defined.

After import, inspect:

```sh
./dist/idios --db ~/.idios/idios-engine.sqlite3 repo list
./dist/idios --db ~/.idios/idios-engine.sqlite3 concept list
./dist/idios --db ~/.idios/idios-engine.sqlite3 issue list --repo 'Legacy Tasks'
./dist/idios --db ~/.idios/idios-engine.sqlite3 graph
```
