-- Private workspace notes belong to IDIOS and never synchronize to a provider.
ALTER TABLE hub_repository_preferences
ADD COLUMN private_note TEXT NOT NULL DEFAULT '';
