schema "public" {
  comment = "standard application schema"
}

schema "application" {
  comment = "standard application schema"
}

# ─────────────────────────────────────────────────────────────────────────────
# platform - event management domain
#
# User identity, authentication and RBAC are owned by Nimbus (see schema-nimbus.hcl
# for the `users` / `accesscontrol` tables). The tables below hold ONLY the event
# management domain. User references (host_user_id, user_id, ...) are foreign keys
# into `application.users`, which Nimbus keeps in sync with Cognito.
# ─────────────────────────────────────────────────────────────────────────────

enum "event_format" {
  schema = schema.application
  values = ["in_person", "virtual", "hybrid"]
}

enum "participation_type" {
  schema = schema.application
  values = ["open", "application", "invite_only", "paid"]
}

enum "event_status" {
  schema = schema.application
  values = ["draft", "pending_approval", "live", "rejected", "completed", "cancelled"]
}

enum "week_status" {
  schema = schema.application
  values = ["draft", "live", "archived", "pending_approval", "rejected"]
}

enum "form_field_type" {
  schema = schema.application
  values = [
    "short_text",
    "long_text",
    "email",
    "phone",
    "single_choice",
    "multi_choice",
    "checkbox",
    "number",
    "date",
    "rating",
    "nps",
    "url",
    "country",
    "city"
  ]
}

enum "registration_status" {
  schema = schema.application
  values = ["pending", "approved", "rejected", "on_hold", "cancelled", "withdrawn"]
}

enum "meeting_status" {
  schema = schema.application
  values = ["requested", "accepted", "declined", "cancelled", "slot_proposed"]
}

enum "communication_type" {
  schema = schema.application
  values = ["lifecycle", "blast", "nudge", "reminder", "feedback_request", "invitation"]
}

enum "communication_status" {
  schema = schema.application
  values = ["draft", "scheduled", "sending", "sent", "failed", "cancelled"]
}

# Records which authority cleared an event to go live. A standalone event is approved by the
# platform superadmin; an event inside a week may instead be approved by that week's host.
enum "approval_authority" {
  schema = schema.application
  values = ["superadmin", "weekhost"]
}

# ── Week Banners ─────────────────────────────────────────────────────────────
# Parent container grouping multiple events (e.g. a "Climate Week"). Tracks shared
# branding and acts as the aggregation root for cross-event analytics.
table "week_banners" {
  schema = schema.application

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "slug" {
    null = false
    type = text
  }
  column "name" {
    null = false
    type = text
  }
  column "description" {
    null = true
    type = text
  }
  column "theme_color" {
    null    = true
    type    = text
    default = "#1f7a4d"
  }
  column "logo_url" {
    null = true
    type = text
  }
  column "start_date" {
    null = true
    type = date
  }
  column "end_date" {
    null = true
    type = date
  }
  column "status" {
    null    = false
    type    = enum.week_status
    default = "draft"
  }
  column "host_user_id" {
    null = true
    type = uuid
  }
  column "about" {
    null    = true
    type    = text
    comment = "Rich-text body for the public week page. `description` stays the short summary."
  }
  column "og_description" {
    null = true
    type = text
  }
  column "cover_image_url" {
    null = true
    type = text
  }
  column "banner_image_url" {
    null = true
    type = text
  }
  column "theme_font" {
    null    = true
    type    = text
    default = "Inter"
  }
  column "city" {
    null = true
    type = text
  }
  column "country" {
    null = true
    type = text
  }
  column "timezone" {
    null    = true
    type    = text
    default = "UTC"
  }
  column "website_url" {
    null = true
    type = text
  }
  column "contact_email" {
    null = true
    type = text
  }
  column "published_at" {
    null = true
    type = timestamptz
  }
  column "rejection_reason" {
    null = true
    type = text
  }
  column "approved_by_user_id" {
    null = true
    type = uuid
  }
  column "approved_at" {
    null = true
    type = timestamptz
  }
  column "approval_authority" {
    null    = true
    type    = enum.approval_authority
    comment = "Always superadmin for weeks today - kept for shape parity with events' approval columns."
  }
  column "withdrawn_at" {
    null    = true
    type    = timestamptz
    comment = "Set when the host pulls their own week submission back to draft. Cleared on resubmit."
  }
  column "withdrawn_from_status" {
    null    = true
    type    = enum.week_status
    comment = "The status the week was in when withdrawn."
  }
  column "changes_requested_at" {
    null    = true
    type    = timestamptz
    comment = "Set when Superadmin asks the host for edits before the week can go live. Cleared on resubmit."
  }
  column "changes_requested_note" {
    null    = true
    type    = text
    comment = "What the approver asked to be changed, shown to the host."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "week_banners_slug_key" {
    columns = [column.slug]
    unique  = true
  }

  index "idx_week_banners_status_start_date" {
    columns = [column.status, column.start_date]
  }

  foreign_key "week_banners_host_user_id_fkey" {
    columns     = [column.host_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "week_banners_approved_by_user_id_fkey" {
    columns     = [column.approved_by_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

# ── Events ───────────────────────────────────────────────────────────────────
# The primary domain object. Config, theming and lifecycle status all live here.
table "events" {
  schema = schema.application

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "slug" {
    null = false
    type = text
  }
  column "week_id" {
    null = true
    type = uuid
  }
  column "title" {
    null = false
    type = text
  }
  column "summary" {
    null = true
    type = text
  }
  column "description" {
    null = true
    type = text
  }
  column "format" {
    null    = false
    type    = enum.event_format
    default = "in_person"
  }
  column "participation_type" {
    null    = false
    type    = enum.participation_type
    default = "open"
  }
  column "capacity" {
    null    = true
    type    = int
    default = 100
  }
  column "theme_color" {
    null    = true
    type    = text
    default = "#1f7a4d"
  }
  column "theme_font" {
    null    = true
    type    = text
    default = "Inter"
  }
  column "cover_image_url" {
    null = true
    type = text
  }
  column "og_description" {
    null = true
    type = text
  }
  column "location" {
    null = true
    type = text
  }
  column "venue_address" {
    null = true
    type = text
  }
  column "banner_image_url" {
    null = true
    type = text
  }
  column "city" {
    null = true
    type = text
  }
  column "state" {
    null = true
    type = text
  }
  column "country" {
    null = true
    type = text
  }
  column "latitude" {
    null = true
    type = float
  }
  column "longitude" {
    null = true
    type = float
  }
  column "sector" {
    null = true
    type = text
  }
  column "climate" {
    null = true
    type = text
  }
  column "target_audience" {
    null = true
    type = text
  }
  column "outcomes" {
    null = true
    type = text
  }
  column "organiser_name" {
    null = true
    type = text
  }
  column "organiser_logo_url" {
    null    = true
    type    = text
    comment = "Square or round host/org mark shown on the public event page."
  }
  column "organiser_logo_shape" {
    null    = true
    type    = text
    default = "round"
    comment = "round | square"
  }
  column "organiser_website_url" {
    null = true
    type = text
  }
  column "organiser_linkedin_url" {
    null = true
    type = text
  }
  column "organiser_social_url" {
    null = true
    type = text
  }
  column "event_tags" {
    null    = true
    type    = text
    default = "[]"
    comment = "JSON-encoded string array of tags, e.g. [\"Networking\"]. Stored as text so GraphJin inserts do not treat the value as a nested relation."
  }
  column "hide_address_until_approved" {
    null    = false
    type    = boolean
    default = false
  }
  column "status" {
    null    = false
    type    = enum.event_status
    default = "draft"
  }
  column "host_user_id" {
    null = false
    type = uuid
  }
  column "start_time" {
    null = true
    type = timestamptz
  }
  column "end_time" {
    null = true
    type = timestamptz
  }
  column "rejection_reason" {
    null = true
    type = text
  }
  column "timezone" {
    null    = true
    type    = text
    default = "UTC"
    comment = "IANA timezone the host authored start_time/end_time in. Recurrence expansion reads this."
  }
  column "published_at" {
    null = true
    type = timestamptz
  }
  column "approved_by_user_id" {
    null = true
    type = uuid
  }
  column "approved_at" {
    null = true
    type = timestamptz
  }
  column "approval_authority" {
    null    = true
    type    = enum.approval_authority
    comment = "Which authority approved this event. NULL until approved."
  }
  column "cancelled_at" {
    null = true
    type = timestamptz
  }
  column "cancellation_reason" {
    null = true
    type = text
  }
  column "withdrawn_at" {
    null    = true
    type    = timestamptz
    comment = "Set when the host pulls their own submission back to draft. Cleared on resubmit."
  }
  column "changes_requested_at" {
    null    = true
    type    = timestamptz
    comment = "Set when Superadmin asks the host for edits on a standalone submission. Cleared on resubmit."
  }
  column "changes_requested_note" {
    null    = true
    type    = text
    comment = "What the approver asked to be changed, shown to the host."
  }
  column "withdrawn_from_status" {
    null    = true
    type    = enum.event_status
    comment = "The status the event was in when withdrawn, so the tracker can show what was pulled back."
  }
  column "cloned_from_event_id" {
    null    = true
    type    = uuid
    comment = "Set when the event was produced by the FR-1.11 clone action. Config only; never registrant data."
  }
  column "series_id" {
    null    = true
    type    = uuid
    comment = "NULL for standalone events. Set on every materialised occurrence of a recurring series."
  }
  column "occurrence_index" {
    null    = true
    type    = int
    comment = "0-based position within the series. Cancelling one occurrence never renumbers its siblings."
  }
  column "waitlist_open" {
    null    = false
    type    = boolean
    default = true
    comment = "When false, registrations at capacity are refused outright instead of waitlisted."
  }
  column "registration_opens_at" {
    null = true
    type = timestamptz
  }
  column "registration_closes_at" {
    null = true
    type = timestamptz
  }
  column "price_amount" {
    null    = true
    type    = decimal(12,2)
    comment = "Only meaningful when participation_type = 'paid'. No payment processing exists yet."
  }
  column "price_currency" {
    null    = true
    type    = text
    default = "USD"
  }
  column "contact_email" {
    null = true
    type = text
  }
  column "virtual_url" {
    null    = true
    type    = text
    comment = "Joining link for virtual and hybrid events (Zoom, Meet, Teams, etc.)."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "events_slug_key" {
    columns = [column.slug]
    unique  = true
  }

  foreign_key "events_week_id_fkey" {
    columns     = [column.week_id]
    ref_columns = [table.week_banners.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  foreign_key "events_host_user_id_fkey" {
    columns     = [column.host_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "events_approved_by_user_id_fkey" {
    columns     = [column.approved_by_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "events_cloned_from_event_id_fkey" {
    columns     = [column.cloned_from_event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  foreign_key "events_series_id_fkey" {
    columns     = [column.series_id]
    ref_columns = [table.event_series.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  index "idx_events_status_start_time" {
    columns = [column.status, column.start_time]
  }

  index "idx_events_week_status" {
    columns = [column.week_id, column.status]
  }

  index "idx_events_series_occurrence" {
    columns = [column.series_id, column.occurrence_index]
  }
}

# ── Event Co-hosts ───────────────────────────────────────────────────────────
# Granular per-event permission set. `permissions` is a JSON map of module ->
# access level (e.g. {"analytics":"view","page":"edit","guests":"none"}).
table "event_cohosts" {
  schema  = schema.application
  comment = "granular co-host permissions scoped to a single event"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "permissions" {
    null    = false
    type    = json
    default = "{}"
  }
  column "granted_by_user_id" {
    null    = true
    type    = uuid
    comment = "The event owner or admin who granted this co-host access."
  }
  column "accepted_at" {
    null = true
    type = timestamptz
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "event_cohosts_event_user_key" {
    columns = [column.event_id, column.user_id]
    unique  = true
  }

  foreign_key "event_cohosts_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "event_cohosts_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "event_cohosts_granted_by_user_id_fkey" {
    columns     = [column.granted_by_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

# ── Dynamic Registration Form Fields ─────────────────────────────────────────
# EAV-style field definitions backing the drag-and-drop form builder. Option lists
# for choice fields are stored as a JSON array on `options`.
table "form_fields" {
  schema  = schema.application
  comment = "dynamic registration form field definitions per event"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "label" {
    null = false
    type = text
  }
  column "field_type" {
    null    = false
    type    = enum.form_field_type
    default = "short_text"
  }
  column "placeholder" {
    null = true
    type = text
  }
  column "options" {
    null    = true
    type    = json
    default = sql("'[]'::json")
    comment = "JSON array of choice options, e.g. [\"Yes\",\"No\"]. Stored as a real json array (was text holding a JSON-encoded array)."
  }
  column "required" {
    null    = false
    type    = boolean
    default = false
  }
  column "is_core" {
    null    = false
    type    = boolean
    default = false
    comment = "the four mandatory fields (name, email, org, stakeholder type) cannot be removed"
  }
  column "help_text" {
    null    = true
    type    = text
    comment = "shown under the label on the public registration form"
  }
  column "is_locked" {
    null    = false
    type    = boolean
    default = false
    comment = "host lock: field cannot be removed until unlocked. Platform core fields are always locked."
  }
  column "position" {
    null    = false
    type    = int
    default = 0
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "form_fields_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── Registrations & Guest List ───────────────────────────────────────────────
table "registrations" {
  schema  = schema.application
  comment = "maps attendees to events, tracking application status and check-in"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null = true
    type = uuid
  }
  column "name" {
    null    = false
    type    = text
    comment = "Display name, kept as \"first last\" so every existing consumer keeps working."
  }
  column "first_name" {
    null    = true
    type    = text
    comment = "Set by forms that collect the name in two parts. NULL on rows predating the split."
  }
  column "last_name" {
    null    = true
    type    = text
    comment = "Set by forms that collect the name in two parts. NULL on rows predating the split."
  }
  column "email" {
    null = false
    type = text
  }
  column "organization" {
    null = true
    type = text
  }
  column "stakeholder_type" {
    null = true
    type = text
  }
  column "status" {
    null    = false
    type    = enum.registration_status
    default = "pending"
  }
  column "responses" {
    null    = true
    type    = json
    default = "{}"
    comment = "answers to the host's custom form fields, keyed by form_field id"
  }
  column "qr_hash" {
    null    = true
    type    = text
    comment = "hashed user_id+event_id, generated on approval"
  }
  column "checked_in" {
    null    = false
    type    = boolean
    default = false
  }
  column "check_in_time" {
    null = true
    type = timestamptz
  }
  column "source" {
    null    = true
    type    = text
    default = "self"
    comment = "self | csv_import | invite"
  }
  column "phone" {
    null = true
    type = text
  }
  column "group_id" {
    null    = true
    type    = uuid
    comment = "Set when this registration was created as part of a group submission (FR-3.6). FK added with registration_groups."
  }
  column "created_by_user_id" {
    null    = true
    type    = uuid
    comment = "The user who submitted this registration if it was not the registrant themselves - group submitter or CSV importer."
  }
  column "is_self_registered" {
    null    = false
    type    = boolean
    default = true
    comment = "False for third-party-supplied registrations. Consent-gated features (showcase, origin map, matchmaking) must exclude these by default."
  }
  column "stakeholder_type_term_id" {
    null    = true
    type    = uuid
    comment = "Structured replacement for the free-text stakeholder_type column. FK added with taxonomy_terms. Both columns coexist during migration."
  }
  column "expected_outcome" {
    null    = true
    type    = text
    comment = "What the registrant wants out of the event. Free text today; the matchmaking scoring model reads this as the seeker-side non-capital-support signal."
  }
  column "cancelled_at" {
    null = true
    type = timestamptz
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "registrations_event_email_key" {
    columns = [column.event_id, column.email]
    unique  = true
  }

  foreign_key "registrations_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "registrations_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "registrations_created_by_user_id_fkey" {
    columns     = [column.created_by_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "registrations_stakeholder_type_term_id_fkey" {
    columns     = [column.stakeholder_type_term_id]
    ref_columns = [table.taxonomy_terms.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "registrations_group_id_fkey" {
    columns     = [column.group_id]
    ref_columns = [table.registration_groups.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  index "idx_registrations_event_status" {
    columns = [column.event_id, column.status]
  }

  index "idx_registrations_user_status" {
    columns = [column.user_id, column.status]
  }
}

# ── Meetings & Scheduling ────────────────────────────────────────────────────
table "meetings" {
  schema  = schema.application
  comment = "15-minute meeting slots between two approved attendees at an event"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "requester_user_id" {
    null = false
    type = uuid
  }
  column "recipient_user_id" {
    null = false
    type = uuid
  }
  column "slot_start" {
    null = false
    type = timestamptz
  }
  column "slot_end" {
    null = false
    type = timestamptz
  }
  column "status" {
    null    = false
    type    = enum.meeting_status
    default = "requested"
  }
  column "message" {
    null = true
    type = text
  }
  column "location" {
    null = true
    type = text
  }
  column "channels" {
    null    = true
    type    = jsonb
    default = "[]"
    comment = "Channels the sender chose, e.g. [\"in_app\",\"email\"]. Dual delivery must not allow a double accept - check notifications.actioned_at before applying."
  }
  column "delivered_at" {
    null = true
    type = timestamptz
  }
  column "responded_at" {
    null = true
    type = timestamptz
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_meetings_event_status" {
    columns = [column.event_id, column.status]
  }

  index "idx_meetings_recipient_status" {
    columns = [column.recipient_user_id, column.status]
  }

  foreign_key "meetings_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "meetings_requester_user_id_fkey" {
    columns     = [column.requester_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "meetings_recipient_user_id_fkey" {
    columns     = [column.recipient_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

# ── Digital Business Cards ───────────────────────────────────────────────────
table "business_cards" {
  schema  = schema.application
  comment = "captured connections between users, logging the event where the scan occurred"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "owner_user_id" {
    null = false
    type = uuid
  }
  column "scanned_user_id" {
    null = true
    type = uuid
  }
  column "event_id" {
    null = true
    type = uuid
  }
  column "display_name" {
    null = true
    type = text
  }
  column "title" {
    null = true
    type = text
  }
  column "organization" {
    null = true
    type = text
  }
  column "email" {
    null = true
    type = text
  }
  column "phone" {
    null = true
    type = text
  }
  column "notes" {
    null = true
    type = text
  }
  column "card_id" {
    null    = true
    type    = uuid
    comment = "The user_business_cards row that was scanned to produce this capture. NULL for manually entered contacts. FK added with user_business_cards."
  }
  column "event_session_id" {
    null    = true
    type    = uuid
    comment = "Optional finer-grained context than event_id. FK added with event_sessions."
  }
  column "capture_source" {
    null    = true
    type    = text
    default = "qr"
    comment = "qr | manual | import"
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "business_cards_owner_user_id_fkey" {
    columns     = [column.owner_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "business_cards_scanned_user_id_fkey" {
    columns     = [column.scanned_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "business_cards_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  foreign_key "business_cards_card_id_fkey" {
    columns     = [column.card_id]
    ref_columns = [table.user_business_cards.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  foreign_key "business_cards_event_session_id_fkey" {
    columns     = [column.event_session_id]
    ref_columns = [table.event_sessions.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  index "idx_business_cards_owner_created" {
    columns = [column.owner_user_id, column.created_at]
  }
}

# ── Communications Log ───────────────────────────────────────────────────────
table "communications" {
  schema  = schema.application
  comment = "tracks lifecycle emails, blast announcements and nudge triggers"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = true
    type = uuid
  }
  column "type" {
    null    = false
    type    = enum.communication_type
    default = "blast"
  }
  column "name" {
    null    = true
    type    = text
    comment = "Internal label for the blast, shown to hosts only. Never sent."
  }
  column "from_email" {
    null    = true
    type    = text
    comment = "Sender address the host picked. NULL falls back to the platform default."
  }
  column "subject" {
    null = false
    type = text
  }
  column "preview_text" {
    null    = true
    type    = text
    comment = "Inbox preview line shown after the subject. Optional."
  }
  column "body" {
    null = true
    type = text
  }
  column "segment" {
    null    = true
    type    = text
    default = "all"
    comment = "target audience segment: all | approved | pending | checked_in"
  }
  column "recipient_count" {
    null    = true
    type    = int
    default = 0
  }
  column "status" {
    null    = false
    type    = enum.communication_status
    default = "draft"
  }
  column "sent_by" {
    null = true
    type = uuid
  }
  column "sent_at" {
    null = true
    type = timestamptz
  }
  column "week_id" {
    null    = true
    type    = uuid
    comment = "Set for a week-level blast. Null together with event_id when audience.eventIds/weekIds lists several targets."
  }
  column "channel" {
    null    = false
    type    = enum.communication_channel
    default = "email"
  }
  column "scheduled_for" {
    null = true
    type = timestamptz
  }
  column "title" {
    null    = true
    type    = text
    comment = "Composer label (Figma Form Name). Subject is what the recipient sees."
  }
  column "sender_email" {
    null    = true
    type    = text
    comment = "Alias of from_email for GraphJin. Do not drop from_email."
  }
  column "audience" {
    null    = true
    type    = jsonb
    default = "{}"
    comment = "Selection snapshot: {eventIds, weekIds, segments}. Used when a blast spans more than one event or week."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "communications_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "communications_sent_by_fkey" {
    columns     = [column.sent_by]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "communications_week_id_fkey" {
    columns     = [column.week_id]
    ref_columns = [table.week_banners.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  index "idx_communications_scheduled" {
    columns = [column.status, column.scheduled_for]
  }
}

# ── Event Checklist ──────────────────────────────────────────────────────────
# State-driven progress tracker from event creation to post-event wrap-up.
table "event_checklist_items" {
  schema  = schema.application
  comment = "host progress checklist, milestones auto-checked as they complete"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "item_key" {
    null = false
    type = text
  }
  column "label" {
    null = false
    type = text
  }
  column "completed" {
    null    = false
    type    = boolean
    default = false
  }
  column "auto" {
    null    = false
    type    = boolean
    default = false
    comment = "true when the milestone is auto-detected rather than manually toggled"
  }
  column "position" {
    null    = false
    type    = int
    default = 0
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "event_checklist_event_key" {
    columns = [column.event_id, column.item_key]
    unique  = true
  }

  foreign_key "event_checklist_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── Phase 2: taxonomy, profiles, consent ─────────────────────────────────────

# One controlled vocabulary table serves several distinct vocabularies. `kind` says which.
# climate_domain   - CCF taxonomy, hierarchical (parent_id), e.g. Decarbonisation > Clean Power Generation
# industry_focus   - cross-sector business classification (Agriculture, Automotive, Healthcare, IT/Software…)
# supply_chain_stage - ORDINAL. `ordinal` carries the stage number; matching logic depends on stage distance.
# startup_stage    - Discovery, Seed, Growth…
# support_offering - non-capital support an institution provides
# sdg_goal         - UN SDGs 1-17; `ordinal` carries the goal number
# stakeholder_type - the registration form's fourth mandatory field, structured
# event_tag        - free-form event tags
# target_audience  - who an event is for
enum "taxonomy_kind" {
  schema = schema.application
  values = [
    "climate_domain",
    "industry_focus",
    "supply_chain_stage",
    "startup_stage",
    "support_offering",
    "sdg_goal",
    "stakeholder_type",
    "event_tag",
    "target_audience"
  ]
}

enum "profile_kind" {
  schema = schema.application
  values = [
    "individual",
    "startup",
    "investor",
    "corporate",
    "service_provider",
    "eso",
    "foundation",
    "government"
  ]
}

# Why a profile is associated with a place. A candidate's stated FOCUS geography is what a
# seeker is matched against - not the candidate's own HQ.
enum "profile_geography_scope" {
  schema = schema.application
  values = ["hq", "operating", "investment_focus", "program_focus"]
}

enum "consent_type" {
  schema = schema.application
  values = [
    "origin_map",
    "attendee_showcase",
    "matchmaking",
    "marketing_email",
    "data_processing",
    "photography",
    "profile_directory"
  ]
}

enum "consent_source" {
  schema = schema.application
  values = ["registration_form", "profile_settings", "invitation_acceptance", "csv_import", "signup"]
}

# ── Controlled vocabularies ──────────────────────────────────────────────────
# Replaces the free-text events.sector / events.climate / events.target_audience columns and the
# events.event_tags text (JSON-encoded string array). Those columns are LEFT IN PLACE and unused so nothing breaks
# during migration - do not drop them.
table "taxonomy_terms" {
  schema  = schema.application
  comment = "single controlled-vocabulary table; `kind` selects the vocabulary"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "kind" {
    null = false
    type = enum.taxonomy_kind
  }
  column "slug" {
    null = false
    type = text
  }
  column "label" {
    null = false
    type = text
  }
  column "description" {
    null = true
    type = text
  }
  column "parent_id" {
    null    = true
    type    = uuid
    comment = "Self-reference for hierarchical vocabularies. A shared granular child scores higher than a shared top-level parent."
  }
  column "ordinal" {
    null    = true
    type    = int
    comment = "Position on an ordered scale. Required for supply_chain_stage (stage distance drives adjacency scoring) and sdg_goal (1-17). NULL for unordered vocabularies."
  }
  column "is_active" {
    null    = false
    type    = boolean
    default = true
  }
  column "position" {
    null    = false
    type    = int
    default = 0
    comment = "Display order within a kind. Distinct from `ordinal`, which is semantic."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "taxonomy_terms_kind_slug_key" {
    columns = [column.kind, column.slug]
    unique  = true
  }

  index "idx_taxonomy_terms_parent_id" {
    columns = [column.parent_id]
  }

  foreign_key "taxonomy_terms_parent_id_fkey" {
    columns     = [column.parent_id]
    ref_columns = [table.taxonomy_terms.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

table "event_taxonomy_terms" {
  schema  = schema.application
  comment = "many-to-many between events and controlled vocabulary terms"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "term_id" {
    null = false
    type = uuid
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "event_taxonomy_terms_event_term_key" {
    columns = [column.event_id, column.term_id]
    unique  = true
  }

  index "idx_event_taxonomy_terms_term_id" {
    columns = [column.term_id]
  }

  foreign_key "event_taxonomy_terms_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "event_taxonomy_terms_term_id_fkey" {
    columns     = [column.term_id]
    ref_columns = [table.taxonomy_terms.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── Profiles ─────────────────────────────────────────────────────────────────
# A platform-domain entity, NOT the Nimbus application.organization table. Covers both the
# individual profile behind a user account (profile_kind = 'individual', user_id NOT NULL) and
# the climate-sector organisations a user represents (all other kinds, user_id = the owner).
#
# The typed columns below are exactly the fields the Phase 2 scoring model reads. Everything
# else - the long tail of section fields per profile kind - lives in `attributes` as jsonb so
# adding a section never needs a migration.
table "profiles" {
  schema  = schema.application
  comment = "individual and organisation profiles; platform domain, unrelated to Nimbus organizations"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "kind" {
    null = false
    type = enum.profile_kind
  }
  column "slug" {
    null = false
    type = text
  }
  column "user_id" {
    null    = true
    type    = uuid
    comment = "For kind='individual' this is the account the profile belongs to. For organisation kinds it is the owner who created it."
  }
  column "display_name" {
    null = false
    type = text
  }
  column "headline" {
    null    = true
    type    = text
    comment = "One-line summary shown in the directory and on business cards."
  }
  column "bio" {
    null = true
    type = text
  }
  column "job_title" {
    null    = true
    type    = text
    comment = "Individual profiles only."
  }
  column "organisation_name" {
    null    = true
    type    = text
    comment = "Free-text employer for an individual who has not linked an organisation profile."
  }
  column "logo_url" {
    null = true
    type = text
  }
  column "photo_url" {
    null = true
    type = text
  }
  column "website_url" {
    null = true
    type = text
  }
  column "contact_email" {
    null = true
    type = text
  }
  column "city" {
    null = true
    type = text
  }
  column "state" {
    null = true
    type = text
  }
  column "country" {
    null    = true
    type    = text
    comment = "HQ country. Match logic compares this against a candidate's stated FOCUS geography in profile_geographies, not against their HQ."
  }
  column "founded_year" {
    null = true
    type = int
  }
  column "team_size" {
    null = true
    type = int
  }
  column "fundraising_active" {
    null    = false
    type    = boolean
    default = false
    comment = "The Fundraising Toggle. Ticket-size matching is only evaluated when this is true."
  }
  column "funding_ask_amount" {
    null    = true
    type    = decimal(16,2)
    comment = "Startup side: the amount being raised."
  }
  column "ticket_size_min" {
    null    = true
    type    = decimal(16,2)
    comment = "Investor side: lower bound of preferred deal size / fund ticket range."
  }
  column "ticket_size_max" {
    null    = true
    type    = decimal(16,2)
  }
  column "currency" {
    null    = true
    type    = text
    default = "USD"
  }
  column "socials" {
    null    = true
    type    = jsonb
    default = "{}"
    comment = "Keyed links: linkedin, x, github, etc."
  }
  column "attributes" {
    null    = true
    type    = jsonb
    default = "{}"
    comment = "Kind-specific section fields that do not drive scoring. Free-form by design - never give this a closed shape."
  }
  column "is_published" {
    null    = false
    type    = boolean
    default = false
    comment = "Unpublished profiles are invisible to the directory and to unauthenticated reads."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "profiles_slug_key" {
    columns = [column.slug]
    unique  = true
  }

  index "idx_profiles_kind_published" {
    columns = [column.kind, column.is_published]
  }

  index "idx_profiles_user_id" {
    columns = [column.user_id]
  }

  foreign_key "profiles_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

table "profile_members" {
  schema  = schema.application
  comment = "users attached to an organisation profile; membership is platform-domain and grants no Nimbus permissions"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "profile_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "role_label" {
    null    = true
    type    = text
    comment = "Display-only, e.g. 'Founder', 'Partner'. Never consulted for authorisation."
  }
  column "is_owner" {
    null    = false
    type    = boolean
    default = false
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "profile_members_profile_user_key" {
    columns = [column.profile_id, column.user_id]
    unique  = true
  }

  foreign_key "profile_members_profile_id_fkey" {
    columns     = [column.profile_id]
    ref_columns = [table.profiles.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "profile_members_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

table "profile_taxonomy_terms" {
  schema  = schema.application
  comment = "multi-select vocabulary tags on a profile; the same term table serves every profile_kind"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "profile_id" {
    null = false
    type = uuid
  }
  column "term_id" {
    null = false
    type = uuid
  }
  column "is_focus" {
    null    = false
    type    = boolean
    default = false
    comment = "True when the term describes what this profile is LOOKING FOR rather than what it IS. An investor's target sectors are focus; a startup's own sector is not."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "profile_taxonomy_terms_profile_term_key" {
    columns = [column.profile_id, column.term_id]
    unique  = true
  }

  index "idx_profile_taxonomy_terms_term_id" {
    columns = [column.term_id]
  }

  foreign_key "profile_taxonomy_terms_profile_id_fkey" {
    columns     = [column.profile_id]
    ref_columns = [table.profiles.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "profile_taxonomy_terms_term_id_fkey" {
    columns     = [column.term_id]
    ref_columns = [table.taxonomy_terms.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

table "profile_geographies" {
  schema  = schema.application
  comment = "countries a profile operates in, invests in, or runs programmes in - separate from its HQ"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "profile_id" {
    null = false
    type = uuid
  }
  column "scope" {
    null = false
    type = enum.profile_geography_scope
  }
  column "country" {
    null    = false
    type    = text
    comment = "ISO 3166-1 alpha-2 where possible."
  }
  column "region" {
    null = true
    type = text
  }
  column "city" {
    null = true
    type = text
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_profile_geographies_profile_scope" {
    columns = [column.profile_id, column.scope]
  }

  index "idx_profile_geographies_country" {
    columns = [column.country]
  }

  foreign_key "profile_geographies_profile_id_fkey" {
    columns     = [column.profile_id]
    ref_columns = [table.profiles.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── Digital business cards ───────────────────────────────────────────────────
# A user's OWN shareable card. The pre-existing `business_cards` table is the opposite thing -
# a log of cards this user CAPTURED from other people. Do not merge them.
table "user_business_cards" {
  schema  = schema.application
  comment = "a user's own shareable card and its QR token; business_cards is the capture log, not this"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "profile_id" {
    null    = true
    type    = uuid
    comment = "Optional source of truth for the card's fields. When set, the card renders live profile data."
  }
  column "share_token" {
    null    = false
    type    = text
    comment = "Opaque token embedded in the QR. Rotating it invalidates every previously shared code."
  }
  column "display_name" {
    null = false
    type = text
  }
  column "title" {
    null = true
    type = text
  }
  column "organisation" {
    null = true
    type = text
  }
  column "email" {
    null = true
    type = text
  }
  column "phone" {
    null = true
    type = text
  }
  column "visible_fields" {
    null    = true
    type    = jsonb
    default = "{}"
    comment = "Per-field visibility map, e.g. {\"phone\":false}. Fields set false are omitted from the scanned payload entirely."
  }
  column "is_active" {
    null    = false
    type    = boolean
    default = true
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "user_business_cards_share_token_key" {
    columns = [column.share_token]
    unique  = true
  }

  index "idx_user_business_cards_user_id" {
    columns = [column.user_id]
  }

  foreign_key "user_business_cards_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "user_business_cards_profile_id_fkey" {
    columns     = [column.profile_id]
    ref_columns = [table.profiles.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }
}

# ── Consent ledger ───────────────────────────────────────────────────────────
# One ledger, not scattered booleans. Every consent-gated feature reads from here.
#
# Both subject columns are nullable because a public event registration needs no account: an
# anonymous registrant consents against their registration_id alone. Exactly one of
# user_id / registration_id must be set - enforced in the resolver, not by a CHECK constraint
# (Atlas check blocks are avoided here to keep the drift gate simple).
#
# text_version is not optional. Proving what someone agreed to requires knowing what they saw.
table "consents" {
  schema  = schema.application
  comment = "append-mostly consent ledger with text version and withdrawal; never store a bare boolean elsewhere"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "consent_type" {
    null = false
    type = enum.consent_type
  }
  column "user_id" {
    null = true
    type = uuid
  }
  column "registration_id" {
    null    = true
    type    = uuid
    comment = "Set for consent captured from an anonymous public registration."
  }
  column "event_id" {
    null    = true
    type    = uuid
    comment = "Scopes the consent to one event where relevant (showcase, origin map). NULL means platform-wide."
  }
  column "granted" {
    null    = false
    type    = boolean
    default = false
  }
  column "text_version" {
    null    = false
    type    = text
    comment = "Identifier of the exact consent copy shown at capture, e.g. 'origin_map.v2'."
  }
  column "source" {
    null = false
    type = enum.consent_source
  }
  column "granted_at" {
    null = true
    type = timestamptz
  }
  column "withdrawn_at" {
    null    = true
    type    = timestamptz
    comment = "Withdrawal takes effect on the next read. Any cached derivative must be invalidated."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_consents_user_type" {
    columns = [column.user_id, column.consent_type]
  }

  index "idx_consents_registration_type" {
    columns = [column.registration_id, column.consent_type]
  }

  foreign_key "consents_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "consents_registration_id_fkey" {
    columns     = [column.registration_id]
    ref_columns = [table.registrations.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "consents_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# Marketing opt-out for class 3. Application-owned. Does not ALTER Nimbus users.
# Lookup is lowercase email. user_id is optional (guests unsub by address).
table "marketing_email_suppressions" {
  schema  = schema.application
  comment = "class 3 opt-out: one row per email, honoured before every blast"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "email" {
    null    = false
    type    = text
    comment = "Stored lowercase. Unique."
  }
  column "user_id" {
    null = true
    type = uuid
  }
  column "source" {
    null    = false
    type    = text
    default = "unsub_link"
    comment = "unsub_link | consent_withdraw | import"
  }
  column "created_at" {
    null    = false
    type    = timestamptz
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "marketing_email_suppressions_email_uniq" {
    unique  = true
    columns = [column.email]
  }

  foreign_key "marketing_email_suppressions_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }
}

# ── Phase 2: week governance ─────────────────────────────────────────────────

enum "week_request_status" {
  schema = schema.application
  values = ["pending", "approved", "rejected", "withdrawn"]
}

# ── Week co-hosts ────────────────────────────────────────────────────────────
# Deliberately mirrors event_cohosts: same JSON permission map shape, same semantics, different
# parent. `permissions` is module -> access level, e.g.
#   {"analytics":"view","page":"edit","events":"edit","comms":"none"}
# A week co-host's reach is bounded by this row. Holding the `cohost` Nimbus role only unlocks
# the PAGES; this table decides which weeks' data is actually readable.
table "week_cohosts" {
  schema  = schema.application
  comment = "granular co-host permissions scoped to a single week banner"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "week_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "permissions" {
    null    = false
    type    = jsonb
    default = "{}"
  }
  column "granted_by_user_id" {
    null    = true
    type    = uuid
    comment = "The week host who granted this access."
  }
  column "accepted_at" {
    null = true
    type = timestamptz
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "week_cohosts_week_user_key" {
    columns = [column.week_id, column.user_id]
    unique  = true
  }

  index "idx_week_cohosts_user_id" {
    columns = [column.user_id]
  }

  foreign_key "week_cohosts_week_id_fkey" {
    columns     = [column.week_id]
    ref_columns = [table.week_banners.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "week_cohosts_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "week_cohosts_granted_by_user_id_fkey" {
    columns     = [column.granted_by_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

# ── Week inclusion requests ──────────────────────────────────────────────────
# events.week_id records the OUTCOME. This table records the REQUEST: who asked, when, who
# decided, and why a rejection happened. A rejected request has no representation in week_id at
# all, which is why the nullable column alone was insufficient.
#
# Approving a request is what sets events.week_id. Withdrawing or rejecting never touches it.
table "week_inclusion_requests" {
  schema  = schema.application
  comment = "an event's request to appear under a week banner, decided by that week's host"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "week_id" {
    null = false
    type = uuid
  }
  column "requested_by_user_id" {
    null = false
    type = uuid
  }
  column "status" {
    null    = false
    type    = enum.week_request_status
    default = "pending"
  }
  column "message" {
    null    = true
    type    = text
    comment = "Optional note from the requesting host to the week host."
  }
  column "decided_by_user_id" {
    null = true
    type = uuid
  }
  column "decided_at" {
    null = true
    type = timestamptz
  }
  column "decision_reason" {
    null    = true
    type    = text
    comment = "Required by convention on rejection; surfaced to the requesting host."
  }
  column "changes_requested_at" {
    null    = true
    type    = timestamptz
    comment = "Set when the week host asks for edits. The request stays pending. Cleared on resubmit."
  }
  column "changes_requested_note" {
    null    = true
    type    = text
    comment = "What the week host asked to be changed, shown to the requesting host."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "week_inclusion_requests_event_week_key" {
    columns = [column.event_id, column.week_id]
    unique  = true
    comment = "One request per event per week. Re-requesting after rejection updates the existing row."
  }

  index "idx_week_inclusion_requests_week_status" {
    columns = [column.week_id, column.status]
  }

  foreign_key "week_inclusion_requests_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "week_inclusion_requests_week_id_fkey" {
    columns     = [column.week_id]
    ref_columns = [table.week_banners.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "week_inclusion_requests_requested_by_user_id_fkey" {
    columns     = [column.requested_by_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "week_inclusion_requests_decided_by_user_id_fkey" {
    columns     = [column.decided_by_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

# ── Phase 2: event content ───────────────────────────────────────────────────

# Who may reach a session's uploaded materials, and when. Defaults to the most restrictive
# useful setting; platform has not yet fixed the policy (see docs open question 3).
enum "material_visibility" {
  schema = schema.application
  values = ["hosts_only", "registered", "approved_only", "checked_in_only", "public"]
}

# Explicit pre-read / post-session discriminator for event_session_materials.
# Previously the kind was reverse-engineered in the frontend from available_from
# (upload-time vs session end), which mis-classified whenever browser and DB clocks
# or timezones disagreed. Keep the kind in the row; available_from stays purely
# "when does this become visible".
enum "material_kind" {
  schema = schema.application
  values = ["pre_read", "post_session"]
}

# ── Recurring events ─────────────────────────────────────────────────────────
# Occurrences are MATERIALISED: expanding the rule writes real `events` rows, each carrying
# series_id and occurrence_index. Nothing here is a virtual view.
#
# Expansion happens in `timezone`, converting to UTC per occurrence - never expand in UTC and
# hope, or occurrences land an hour off across a DST boundary.
#
# Expansion must be BOUNDED. Either ends_on or occurrence_count must be set; an unbounded rule
# generates rows until the process dies. Enforce in the resolver.
table "event_series" {
  schema  = schema.application
  comment = "recurrence definition for a materialised series of events"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "owner_user_id" {
    null = false
    type = uuid
  }
  column "name" {
    null = true
    type = text
  }
  column "recurrence_rule" {
    null    = false
    type    = text
    comment = "RFC 5545 RRULE string. Must round-trip as valid RRULE so ICS export and calendar sync stay possible without a second format."
  }
  column "timezone" {
    null    = false
    type    = text
    default = "UTC"
    comment = "IANA zone the rule is expanded in."
  }
  column "starts_on" {
    null = false
    type = date
  }
  column "ends_on" {
    null    = true
    type    = date
    comment = "Either this or occurrence_count must be set. Both NULL is an unbounded rule and must be rejected."
  }
  column "occurrence_count" {
    null = true
    type = int
  }
  column "template_event_id" {
    null    = true
    type    = uuid
    comment = "The occurrence whose configuration is copied into generated siblings. Registrant data is never copied - reuse the FR-1.11 clone deny-list."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_event_series_owner_user_id" {
    columns = [column.owner_user_id]
  }

  foreign_key "event_series_owner_user_id_fkey" {
    columns     = [column.owner_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "event_series_template_event_id_fkey" {
    columns     = [column.template_event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }
}

# ── Speakers ─────────────────────────────────────────────────────────────────
# user_id is NULLABLE by design: an external speaker has no platform account and the host types
# everything in. When user_id IS set, the resolver must pick one source of truth and stick to it.
# The rule chosen here: STORED COLUMNS ALWAYS WIN for display. A linked profile only supplies the
# deep link. This keeps a speaker's event-page bio stable when they later edit their profile.
# Document that rule in a comment at the resolver too.
table "event_speakers" {
  schema  = schema.application
  comment = "speaker profiles on an event; user_id nullable for external speakers"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null    = true
    type    = uuid
    comment = "Set only when the speaker is a registered platform user. Removing a speaker never touches the linked account."
  }
  column "profile_id" {
    null    = true
    type    = uuid
    comment = "Optional link to a platform profile for the deep link on the public page."
  }
  column "display_name" {
    null = false
    type = text
  }
  column "title" {
    null = true
    type = text
  }
  column "organisation" {
    null = true
    type = text
  }
  column "bio" {
    null = true
    type = text
  }
  column "photo_url" {
    null = true
    type = text
  }
  column "socials" {
    null    = true
    type    = jsonb
    default = "{}"
  }
  column "position" {
    null    = false
    type    = int
    default = 0
    comment = "Host-controlled ordering. Reordering is a single batched transaction - never one row at a time, or the page shows intermediate states."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_event_speakers_event_position" {
    columns = [column.event_id, column.position]
  }

  index "idx_event_speakers_user_id" {
    columns = [column.user_id]
  }

  foreign_key "event_speakers_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "event_speakers_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  foreign_key "event_speakers_profile_id_fkey" {
    columns     = [column.profile_id]
    ref_columns = [table.profiles.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }
}

# ── Sponsors ─────────────────────────────────────────────────────────────────
# Same lifecycle as speakers (added on demand through the event form, host-ordered, public on the
# event page) but a different field set - tier, logo and website instead of bio and title. Kept
# separate from event_speakers on purpose.
table "event_sponsors" {
  schema  = schema.application
  comment = "sponsor entries on an event; profile_id nullable for sponsors with no platform profile"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "profile_id" {
    null = true
    type = uuid
  }
  column "name" {
    null = false
    type = text
  }
  column "tier" {
    null    = true
    type    = text
    comment = "Free text, host-defined, e.g. 'Platinum'. Not an enum - tier names differ per event."
  }
  column "description" {
    null = true
    type = text
  }
  column "logo_url" {
    null = true
    type = text
  }
  column "website_url" {
    null = true
    type = text
  }
  column "contact_email" {
    null = true
    type = text
  }
  column "position" {
    null    = false
    type    = int
    default = 0
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_event_sponsors_event_position" {
    columns = [column.event_id, column.position]
  }

  foreign_key "event_sponsors_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "event_sponsors_profile_id_fkey" {
    columns     = [column.profile_id]
    ref_columns = [table.profiles.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }
}

# ── Sessions ─────────────────────────────────────────────────────────────────
# Sessions give an event an internal schedule, which introduces intra-event clash: two sessions
# an attendee wants, running at once. The existing clash detection (FR-17.3) works across events
# and cannot see inside one. The data model here supports session-level clash detection whether
# or not the UI ships it.
table "event_sessions" {
  schema  = schema.application
  comment = "agenda sessions within an event, each with its own time window"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "title" {
    null = false
    type = text
  }
  column "description" {
    null = true
    type = text
  }
  column "notes" {
    null    = true
    type    = text
    comment = "Organiser notes shown alongside the session on the attendee-facing agenda."
  }
  column "start_time" {
    null = true
    type = timestamptz
  }
  column "end_time" {
    null = true
    type = timestamptz
  }
  column "room" {
    null    = true
    type    = text
    comment = "Room, stage or track label within the venue."
  }
  column "position" {
    null    = false
    type    = int
    default = 0
    comment = "Explicit ordering for sessions with no time set. Sessions with times display in time order."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_event_sessions_event_start_time" {
    columns = [column.event_id, column.start_time]
  }

  foreign_key "event_sessions_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# Session speakers REFERENCE event_speakers rows. They never store their own name or bio - if
# they did, the same person would appear twice on one event page with two different bios.
table "event_session_speakers" {
  schema  = schema.application
  comment = "join between sessions and the event's speaker records; never duplicates speaker data"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "session_id" {
    null = false
    type = uuid
  }
  column "speaker_id" {
    null = false
    type = uuid
  }
  column "role_label" {
    null    = true
    type    = text
    comment = "e.g. 'Moderator', 'Panellist'. Display only."
  }
  column "position" {
    null    = false
    type    = int
    default = 0
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "event_session_speakers_session_speaker_key" {
    columns = [column.session_id, column.speaker_id]
    unique  = true
  }

  foreign_key "event_session_speakers_session_id_fkey" {
    columns     = [column.session_id]
    ref_columns = [table.event_sessions.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "event_session_speakers_speaker_id_fkey" {
    columns     = [column.speaker_id]
    ref_columns = [table.event_speakers.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── Session materials ────────────────────────────────────────────────────────
# Presentation decks are the largest objects on the platform - tens of megabytes each.
#
# file_id has NO foreign key on purpose: it points at core.file_object, which GraphJin cannot
# see. The denormalised file_name / file_size_bytes / mime_type columns exist so the agenda
# renders from one GraphQL query, and the frontend only calls
# GET /api/v1/file/download/{fileId} when the user actually clicks download.
#
# Because there is no FK, deleting a material row does NOT delete the underlying object. Orphan
# cleanup is an application concern - delete the core file first, then the row.
#
# Access is authorised server-side against `visibility`, never by an unlisted URL.
table "event_session_materials" {
  schema  = schema.application
  comment = "files attached to a session; file_id references core.file_object without a FK (GraphJin cannot see core)"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "session_id" {
    null = false
    type = uuid
  }
  column "title" {
    null = false
    type = text
  }
  column "description" {
    null = true
    type = text
  }
  column "file_id" {
    null    = true
    type    = uuid
    comment = "core.file_object id. Intentionally NOT a foreign key - cross-schema relations are invisible to GraphJin."
  }
  column "file_name" {
    null    = true
    type    = text
    comment = "Denormalised from core.file_object so the agenda renders without a REST round-trip."
  }
  column "file_size_bytes" {
    null = true
    type = bigint
  }
  column "mime_type" {
    null = true
    type = text
  }
  column "external_url" {
    null    = true
    type    = text
    comment = "Alternative to file_id for materials hosted elsewhere, e.g. a recording link."
  }
  column "visibility" {
    null    = false
    type    = enum.material_visibility
    default = "approved_only"
  }
  column "material_kind" {
    null    = false
    type    = enum.material_kind
    default = "pre_read"
    comment = "pre_read or post_session. Explicit column - the kind must not be inferred from available_from."
  }
  column "available_from" {
    null    = true
    type    = timestamptz
    comment = "NULL means available as soon as it is uploaded."
  }
  column "available_until" {
    null    = true
    type    = timestamptz
    comment = "NULL means indefinitely. Post-event access windows are set here, not in code."
  }
  column "position" {
    null    = false
    type    = int
    default = 0
  }
  column "uploaded_by_user_id" {
    null = true
    type = uuid
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_event_session_materials_session_position" {
    columns = [column.session_id, column.position]
  }

  foreign_key "event_session_materials_session_id_fkey" {
    columns     = [column.session_id]
    ref_columns = [table.event_sessions.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "event_session_materials_uploaded_by_user_id_fkey" {
    columns     = [column.uploaded_by_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

# ── Phase 2: capacity, tickets, origins ──────────────────────────────────────

enum "waitlist_status" {
  schema = schema.application
  values = ["waiting", "promoted", "accepted", "expired", "declined"]
}

enum "ticket_scope" {
  schema = schema.application
  values = ["event", "week"]
}

enum "import_status" {
  schema = schema.application
  values = ["pending", "processing", "completed", "partial", "failed"]
}

enum "import_row_status" {
  schema = schema.application
  values = ["pending", "created", "skipped", "failed"]
}

# ── Waitlist ─────────────────────────────────────────────────────────────────
# Lifecycle: waiting → promoted → accepted | expired | declined.
#
# A seat freeing up promotes the lowest-position waiting row, sets promotion_expires_at, and
# notifies. If that promotion expires or is declined, the next promotion fires immediately - no
# seat sits empty while entries remain.
#
# `position` is per-event and monotonic. It is never renumbered; gaps left by departures are
# expected and harmless.
table "event_waitlist" {
  schema  = schema.application
  comment = "FIFO waitlist for events at capacity; separate from registrations by design"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "email" {
    null    = false
    type    = text
    comment = "Captured at join time so a notification can be sent even if the account is later removed."
  }
  column "status" {
    null    = false
    type    = enum.waitlist_status
    default = "waiting"
  }
  column "position" {
    null    = false
    type    = int
    comment = "Assigned as COALESCE(MAX(position),0)+1 inside the same FOR UPDATE transaction that checks capacity."
  }
  column "promoted_at" {
    null = true
    type = timestamptz
  }
  column "promotion_expires_at" {
    null    = true
    type    = timestamptz
    comment = "How long a promoted entry has to accept. Per-event configurable rather than a global constant."
  }
  column "responded_at" {
    null = true
    type = timestamptz
  }
  column "registration_id" {
    null    = true
    type    = uuid
    comment = "The registration created when this entry was accepted. NULL in every other state."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "event_waitlist_event_user_key" {
    columns = [column.event_id, column.user_id]
    unique  = true
  }

  index "idx_event_waitlist_event_status" {
    columns = [column.event_id, column.status]
  }

  foreign_key "event_waitlist_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "event_waitlist_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "event_waitlist_registration_id_fkey" {
    columns     = [column.registration_id]
    ref_columns = [table.registrations.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }
}

# ── Group registration ───────────────────────────────────────────────────────
# The group is METADATA, never a status container. Approval, rejection, check-in and cancellation
# all operate per registration. A group with three approved and two rejected members is normal.
table "registration_groups" {
  schema  = schema.application
  comment = "one submission creating N registrations; per-guest status stays independent"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "submitter_user_id" {
    null = true
    type = uuid
  }
  column "submitter_email" {
    null = false
    type = text
  }
  column "group_size" {
    null = false
    type = int
  }
  column "submitted_at" {
    null    = false
    type    = timestamptz
    default = sql("CURRENT_TIMESTAMP")
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_registration_groups_event_id" {
    columns = [column.event_id]
  }

  foreign_key "registration_groups_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "registration_groups_submitter_user_id_fkey" {
    columns     = [column.submitter_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

# ── Bulk import ──────────────────────────────────────────────────────────────
# A 1000-row CSV cannot be a single synchronous request. The import row carries job state; the
# per-row table carries the outcome and the error, so a host can fix twelve bad rows instead of
# re-uploading the whole file.
#
# Every registration created this way gets is_self_registered = false - third-party-supplied data
# is excluded from consent-gated features by default.
table "registration_imports" {
  schema  = schema.application
  comment = "bulk CSV/Excel import job for an event's guest list"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "imported_by_user_id" {
    null = false
    type = uuid
  }
  column "file_id" {
    null    = true
    type    = uuid
    comment = "core.file_object id of the uploaded source file. Intentionally not a foreign key."
  }
  column "file_name" {
    null = true
    type = text
  }
  column "status" {
    null    = false
    type    = enum.import_status
    default = "pending"
  }
  column "total_rows" {
    null    = false
    type    = int
    default = 0
  }
  column "created_rows" {
    null    = false
    type    = int
    default = 0
  }
  column "failed_rows" {
    null    = false
    type    = int
    default = 0
  }
  column "error_summary" {
    null = true
    type = text
  }
  column "started_at" {
    null = true
    type = timestamptz
  }
  column "completed_at" {
    null = true
    type = timestamptz
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_registration_imports_event_status" {
    columns = [column.event_id, column.status]
  }

  foreign_key "registration_imports_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "registration_imports_imported_by_user_id_fkey" {
    columns     = [column.imported_by_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

table "registration_import_rows" {
  schema  = schema.application
  comment = "per-row outcome of a bulk import so hosts can fix only what failed"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "import_id" {
    null = false
    type = uuid
  }
  column "row_number" {
    null = false
    type = int
  }
  column "raw_data" {
    null    = true
    type    = jsonb
    default = "{}"
    comment = "The source row exactly as parsed. Free-form by design."
  }
  column "status" {
    null    = false
    type    = enum.import_row_status
    default = "pending"
  }
  column "registration_id" {
    null = true
    type = uuid
  }
  column "error_message" {
    null    = true
    type    = text
    comment = "Field-level and specific, e.g. 'row 42: email already registered for this event'."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_registration_import_rows_import_status" {
    columns = [column.import_id, column.status]
  }

  foreign_key "registration_import_rows_import_id_fkey" {
    columns     = [column.import_id]
    ref_columns = [table.registration_imports.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "registration_import_rows_registration_id_fkey" {
    columns     = [column.registration_id]
    ref_columns = [table.registrations.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }
}

# ── Tickets ──────────────────────────────────────────────────────────────────
# One token can cover a single event OR every one of an attendee's registrations within a week.
# Week-scoped issuance happens on the attendee's FIRST approval inside that week; later approvals
# EXTEND the existing token's reach rather than issuing a new code.
#
# Exactly one of event_id / week_id is set, matching `scope`. Enforced in the resolver.
#
# Revocation: bump `version` and rewrite token_hash. Any previously shared QR stops resolving.
# The pre-existing registrations.qr_hash column stays for backward compatibility during
# migration - treat it as deprecated and do not add new readers.
table "event_tickets" {
  schema  = schema.application
  comment = "QR ticket tokens, scoped to one event or to a whole week"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "scope" {
    null = false
    type = enum.ticket_scope
  }
  column "event_id" {
    null    = true
    type    = uuid
    comment = "Set when scope = 'event'."
  }
  column "week_id" {
    null    = true
    type    = uuid
    comment = "Set when scope = 'week'."
  }
  column "token_hash" {
    null    = false
    type    = text
    comment = "Hash of the opaque token embedded in the QR. The plaintext token is never stored."
  }
  column "version" {
    null    = false
    type    = int
    default = 1
    comment = "Incremented on reissue. A scanned token carrying an older version is rejected."
  }
  column "issued_at" {
    null    = false
    type    = timestamptz
    default = sql("CURRENT_TIMESTAMP")
  }
  column "expires_at" {
    null = true
    type = timestamptz
  }
  column "revoked_at" {
    null = true
    type = timestamptz
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "event_tickets_token_hash_key" {
    columns = [column.token_hash]
    unique  = true
  }

  index "idx_event_tickets_user_scope" {
    columns = [column.user_id, column.scope]
  }

  index "idx_event_tickets_event_id" {
    columns = [column.event_id]
  }

  foreign_key "event_tickets_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "event_tickets_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "event_tickets_week_id_fkey" {
    columns     = [column.week_id]
    ref_columns = [table.week_banners.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── Check-in scan log ────────────────────────────────────────────────────────
# registrations.checked_in / check_in_time record the CURRENT state. This records every attempt,
# including the failures, which is what makes a disputed check-in resolvable and what feeds the
# live attendance metric.
#
# A week-scoped token scanned by an operator assigned to one event must resolve to that event's
# registration or return an explicit disambiguation. It must never guess - checking someone into
# the wrong event corrupts attendance invisibly.
table "ticket_scans" {
  schema  = schema.application
  comment = "append-only log of every check-in scan attempt, successful or not"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null    = false
    type    = uuid
    comment = "The event the scanner was operating for, which is not necessarily the token's scope."
  }
  column "ticket_id" {
    null = true
    type = uuid
  }
  column "registration_id" {
    null    = true
    type    = uuid
    comment = "NULL when resolution failed."
  }
  column "scanned_by_user_id" {
    null = false
    type = uuid
  }
  column "result" {
    null    = false
    type    = text
    comment = "ok | already_checked_in | not_approved | wrong_event | revoked | unknown_token | ambiguous"
  }
  column "scanned_at" {
    null    = false
    type    = timestamptz
    default = sql("CURRENT_TIMESTAMP")
  }
  column "device_label" {
    null = true
    type = text
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_ticket_scans_event_scanned_at" {
    columns = [column.event_id, column.scanned_at]
  }

  index "idx_ticket_scans_registration_id" {
    columns = [column.registration_id]
  }

  foreign_key "ticket_scans_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "ticket_scans_ticket_id_fkey" {
    columns     = [column.ticket_id]
    ref_columns = [table.event_tickets.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  foreign_key "ticket_scans_registration_id_fkey" {
    columns     = [column.registration_id]
    ref_columns = [table.registrations.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  foreign_key "ticket_scans_scanned_by_user_id_fkey" {
    columns     = [column.scanned_by_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

# ── Attendee origin map ──────────────────────────────────────────────────────
# ⚠️ PRECISION IS A PRIVACY DECISION, NOT A DEFAULT.
# latitude/longitude are here so the map can render, but they must be written ROUNDED to the
# agreed precision (2dp ≈ 1 km) at insert time. Never store precise coordinates and coarsen at
# render - the precise data still exists and can leak through any new query path.
# If platform settles on city-level only, DELETE the latitude and longitude columns before
# shipping rather than leaving them nullable.
#
# Consent lives in the `consents` ledger (consent_type = 'origin_map'), not here. A row existing
# in this table is not permission to display it - non-consenting attendees must be ABSENT from
# the response, not present and flagged.
table "registration_origins" {
  schema  = schema.application
  comment = "coarse geographic origin of a registration, for the attendee origin map"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "registration_id" {
    null = false
    type = uuid
  }
  column "city" {
    null = true
    type = text
  }
  column "region" {
    null = true
    type = text
  }
  column "country" {
    null = false
    type = text
  }
  column "latitude" {
    null    = true
    type    = decimal(8,2)
    comment = "Rounded at write time. decimal(8,2) makes storing finer precision structurally impossible."
  }
  column "longitude" {
    null    = true
    type    = decimal(8,2)
  }
  column "resolved_from" {
    null    = true
    type    = text
    comment = "registration_fields | profile_location | manual"
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "registration_origins_registration_id_key" {
    columns = [column.registration_id]
    unique  = true
  }

  index "idx_registration_origins_country_city" {
    columns = [column.country, column.city]
  }

  foreign_key "registration_origins_registration_id_fkey" {
    columns     = [column.registration_id]
    ref_columns = [table.registrations.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── Geocode cache ────────────────────────────────────────────────────────────
# Shared by the venue map picker (FR-1.2) and the origin map (FR-18): same provider, same cache,
# same key. Geocode once per distinct place string, never once per attendee.
#
# Nimbus offers no geocoding surface - the provider is called from a Next.js route handler and the
# result is written here.
table "geocode_cache" {
  schema  = schema.application
  comment = "provider-agnostic geocoding cache shared by the venue picker and the origin map"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "query_hash" {
    null    = false
    type    = text
    comment = "Stable hash of the normalised query string. The cache key."
  }
  column "query_text" {
    null = false
    type = text
  }
  column "provider" {
    null = false
    type = text
  }
  column "latitude" {
    null = true
    type = decimal(10,6)
  }
  column "longitude" {
    null = true
    type = decimal(10,6)
  }
  column "city" {
    null = true
    type = text
  }
  column "region" {
    null = true
    type = text
  }
  column "country" {
    null = true
    type = text
  }
  column "raw_response" {
    null    = true
    type    = jsonb
    default = "{}"
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "geocode_cache_query_provider_key" {
    columns = [column.query_hash, column.provider]
    unique  = true
  }
}

# ── Phase 2: communications, notifications, feedback ─────────────────────────

enum "communication_channel" {
  schema  = schema.application
  values  = ["email", "in_app", "whatsapp"]
  comment = "whatsapp is declared for FR-5.7 but has no Nimbus surface; nothing implements it yet"
}

# delivered / bounced / opened are declared for forward compatibility only. Nimbus email returns
# a bare 200 with no message id and no webhooks, so only queued / sent / failed are writable today.
enum "delivery_status" {
  schema = schema.application
  values = ["queued", "sent", "failed", "delivered", "bounced", "opened"]
}

enum "scheduled_job_status" {
  schema = schema.application
  values = ["pending", "running", "completed", "failed", "cancelled"]
}

enum "feedback_audience" {
  schema = schema.application
  values = ["all_registered", "all_approved", "checked_in_only"]
}

enum "feedback_form_status" {
  schema = schema.application
  values = ["draft", "scheduled", "sent", "closed"]
}

# ── Templates ────────────────────────────────────────────────────────────────
# A template can be a platform default (event_id NULL, trigger_key set) or a host's override for
# one event. Resolution order: event override → platform default for that trigger → hard failure.
table "communication_templates" {
  schema  = schema.application
  comment = "email/notification templates; platform defaults plus per-event host overrides"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null    = true
    type    = uuid
    comment = "NULL means this is the platform default for its trigger_key."
  }
  column "trigger_key" {
    null    = false
    type    = text
    comment = "Stable key, e.g. registration.confirmed, registration.rejected, waitlist.promoted, event.cancelled, feedback.request."
  }
  column "channel" {
    null    = false
    type    = enum.communication_channel
    default = "email"
  }
  column "name" {
    null = false
    type = text
  }
  column "subject" {
    null = true
    type = text
  }
  column "body" {
    null = false
    type = text
  }
  column "body_type" {
    null    = false
    type    = text
    default = "text/html"
  }
  column "from_email" {
    null    = true
    type    = text
    comment = "Sender address for this template. NULL falls back to the event contact address."
  }
  column "preview_text" {
    null = true
    type = text
  }
  column "audience_segment" {
    null    = true
    type    = text
    comment = "Registration segment this template targets, matching the blast composer audience keys (e.g. approved, all). NULL means the trigger determines the audience."
  }
  column "variables" {
    null    = true
    type    = jsonb
    default = "{}"
    comment = "Documented placeholder names available to this template. Free-form by design."
  }
  column "is_active" {
    null    = false
    type    = boolean
    default = true
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "communication_templates_event_trigger_key" {
    columns = [column.event_id, column.trigger_key]
    unique  = true
  }

  foreign_key "communication_templates_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── Per-recipient delivery ledger ────────────────────────────────────────────
# The existing `communications` table records that a blast was composed. It has no per-recipient
# rows, so nothing can currently answer "did this person get it" or compute a response rate.
# This is that ledger.
#
# recipient_email is stored rather than joined so a send is auditable after an account is deleted.
table "communication_recipients" {
  schema  = schema.application
  comment = "one row per recipient per communication; the only source of send/failure truth"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "communication_id" {
    null = false
    type = uuid
  }
  column "registration_id" {
    null = true
    type = uuid
  }
  column "user_id" {
    null = true
    type = uuid
  }
  column "recipient_email" {
    null = false
    type = text
  }
  column "channel" {
    null    = false
    type    = enum.communication_channel
    default = "email"
  }
  column "status" {
    null    = false
    type    = enum.delivery_status
    default = "queued"
  }
  column "sent_at" {
    null = true
    type = timestamptz
  }
  column "failed_at" {
    null = true
    type = timestamptz
  }
  column "error_message" {
    null = true
    type = text
  }
  column "attempts" {
    null    = false
    type    = int
    default = 0
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_communication_recipients_comm_status" {
    columns = [column.communication_id, column.status]
  }

  index "idx_communication_recipients_registration_id" {
    columns = [column.registration_id]
  }

  index "communication_recipients_comm_reg_uniq" {
    unique  = true
    columns = [column.communication_id, column.registration_id]
    where   = "registration_id IS NOT NULL"
  }

  foreign_key "communication_recipients_communication_id_fkey" {
    columns     = [column.communication_id]
    ref_columns = [table.communications.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "communication_recipients_registration_id_fkey" {
    columns     = [column.registration_id]
    ref_columns = [table.registrations.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  foreign_key "communication_recipients_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

# ── Reminder and lifecycle schedules ─────────────────────────────────────────
# Configurable per event: "1 hour before", "1 day before", "3 hours after". offset_minutes is
# signed and relative to `anchor` - negative is before, positive is after.
table "communication_schedules" {
  schema  = schema.application
  comment = "when a template fires relative to an event anchor; drained by the scheduled_jobs ticker"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "template_id" {
    null = true
    type = uuid
  }
  column "anchor" {
    null    = false
    type    = text
    default = "event_start"
    comment = "event_start | event_end | registration_created | registration_approved"
  }
  column "offset_minutes" {
    null    = false
    type    = int
    default = 0
    comment = "Signed. -60 fires an hour before the anchor; +180 fires three hours after."
  }
  column "audience" {
    null    = false
    type    = enum.feedback_audience
    default = "all_approved"
    comment = "Reuses the feedback audience vocabulary - the segments are identical."
  }
  column "channel" {
    null    = false
    type    = enum.communication_channel
    default = "email"
  }
  column "is_active" {
    null    = false
    type    = boolean
    default = true
  }
  column "last_run_at" {
    null = true
    type = timestamptz
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_communication_schedules_event_active" {
    columns = [column.event_id, column.is_active]
  }

  foreign_key "communication_schedules_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "communication_schedules_template_id_fkey" {
    columns     = [column.template_id]
    ref_columns = [table.communication_templates.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }
}

# ── Scheduling substrate ─────────────────────────────────────────────────────
# This table is a platform queue, not the Nimbus clock. Nimbus v0.3.4-snapshot
# registers HTTP routes from crons.json with EventBridge Scheduler; that ticker
# does not drain scheduled_jobs. Something must still poll this table for waitlist
# expiry / nudges if those paths use it. Notification mail uses
# application.notification_dispatches + POST /api/cron/dispatch instead.

#
# Claim protocol: SELECT … WHERE status='pending' AND next_run_at <= now() ORDER BY next_run_at
# FOR UPDATE SKIP LOCKED, then set status='running' and locked_at. A row whose locked_at is older
# than the lease window is considered abandoned and may be reclaimed.
#
# Drives: waitlist promotion expiry sweeps, reminder sends, post-event feedback dispatch, nudges.
table "scheduled_jobs" {
  schema  = schema.application
  comment = "due-work queue drained by an external ticker; Nimbus provides no scheduler"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "job_type" {
    null    = false
    type    = text
    comment = "waitlist.expiry_sweep | communication.scheduled_send | feedback.dispatch | nudge.evaluate"
  }
  column "payload" {
    null    = true
    type    = jsonb
    default = "{}"
    comment = "Job arguments. Free-form by design - never give this a closed shape."
  }
  column "status" {
    null    = false
    type    = enum.scheduled_job_status
    default = "pending"
  }
  column "next_run_at" {
    null    = false
    type    = timestamptz
    comment = "Due time. The ticker claims rows where this is in the past."
  }
  column "locked_at" {
    null    = true
    type    = timestamptz
    comment = "Set when a worker claims the row. Stale locks are reclaimable after the lease window."
  }
  column "attempts" {
    null    = false
    type    = int
    default = 0
  }
  column "max_attempts" {
    null    = false
    type    = int
    default = 3
  }
  column "last_error" {
    null = true
    type = text
  }
  column "completed_at" {
    null = true
    type = timestamptz
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_scheduled_jobs_status_next_run_at" {
    columns = [column.status, column.next_run_at]
  }

  index "idx_scheduled_jobs_job_type" {
    columns = [column.job_type]
  }
}

# ── In-app notifications ─────────────────────────────────────────────────────
# Deliberately DOMAIN-AGNOSTIC. Meeting requests need it, targeted announcements need it, and
# condition-triggered nudges need it. A meeting-request-specific table would mean writing this
# twice more.
#
# `payload` carries whatever the type needs - no per-type columns, ever.
table "notifications" {
  schema  = schema.application
  comment = "generic in-app notification inbox; no domain-specific assumptions"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "type" {
    null    = false
    type    = text
    comment = "meeting.requested | meeting.accepted | event.changed | event.cancelled | waitlist.promoted | checklist.reminder | announcement"
  }
  column "title" {
    null = false
    type = text
  }
  column "body" {
    null = true
    type = text
  }
  column "payload" {
    null    = true
    type    = jsonb
    default = "{}"
    comment = "Type-specific data including any deep-link target. Free-form by design."
  }
  column "event_id" {
    null    = true
    type    = uuid
    comment = "Optional scoping so an event's notifications can be filtered or cleaned up together."
  }
  column "read_at" {
    null = true
    type = timestamptz
  }
  column "actioned_at" {
    null    = true
    type    = timestamptz
    comment = "Set when the user completed the action the notification asked for. Acting from email and then in-app must be idempotent - check this before applying an action twice."
  }
  column "dismissed_at" {
    null = true
    type = timestamptz
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_notifications_user_created_at" {
    columns = [column.user_id, column.created_at]
  }

  index "idx_notifications_user_read_at" {
    columns = [column.user_id, column.read_at]
  }

  foreign_key "notifications_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  foreign_key "notifications_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── Post-event feedback ──────────────────────────────────────────────────────
# form_definition reuses the shape of the FR-3.3 form builder. The `rating` and `nps` field types
# were appended to form_field_type in prompt 02 specifically so this can share that model rather
# than fork it.
table "event_feedback_forms" {
  schema  = schema.application
  comment = "one feedback form per event, scheduled relative to event end"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "title" {
    null = false
    type = text
  }
  column "form_definition" {
    null    = true
    type    = jsonb
    default = "[]"
    comment = "Ordered array of field definitions mirroring form_fields. Free-form by design."
  }
  column "is_anonymous" {
    null    = false
    type    = boolean
    default = true
    comment = "When true, responses carry NO registration link anywhere - not in the row, not in logs. Response rate still works because dispatches are counted separately."
  }
  column "audience" {
    null    = false
    type    = enum.feedback_audience
    default = "all_approved"
  }
  column "send_offset_hours" {
    null    = false
    type    = int
    default = 24
    comment = "Hours after event end to dispatch."
  }
  column "status" {
    null    = false
    type    = enum.feedback_form_status
    default = "draft"
  }
  column "closes_at" {
    null = true
    type = timestamptz
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "event_feedback_forms_event_id_key" {
    columns = [column.event_id]
    unique  = true
  }

  foreign_key "event_feedback_forms_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── The dispatch ledger ──────────────────────────────────────────────────────
# THIS IS THE TABLE THAT MAKES ANONYMOUS RESPONSE RATES POSSIBLE, and it cannot be added
# retroactively. Response rate = COUNT(responses) / COUNT(dispatches), computed without ever
# attributing an individual response to a person.
#
# The dispatch row knows who was asked. The response row does not know who answered.
table "event_feedback_dispatches" {
  schema  = schema.application
  comment = "who a feedback form was sent to; never joined to responses when the form is anonymous"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "form_id" {
    null = false
    type = uuid
  }
  column "registration_id" {
    null = false
    type = uuid
  }
  column "dispatched_at" {
    null = true
    type = timestamptz
  }
  column "status" {
    null    = false
    type    = enum.delivery_status
    default = "queued"
  }
  column "responded" {
    null    = false
    type    = boolean
    default = false
    comment = "A bare boolean, deliberately. It marks that this dispatch produced a response WITHOUT pointing at which one."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "event_feedback_dispatches_form_registration_key" {
    columns = [column.form_id, column.registration_id]
    unique  = true
  }

  foreign_key "event_feedback_dispatches_form_id_fkey" {
    columns     = [column.form_id]
    ref_columns = [table.event_feedback_forms.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "event_feedback_dispatches_registration_id_fkey" {
    columns     = [column.registration_id]
    ref_columns = [table.registrations.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── Responses ────────────────────────────────────────────────────────────────
# registration_id is NULLABLE and MUST be left NULL for anonymous forms - in the row, in every
# log line, and in every API payload. If an admin can map a response back to a person, anonymity
# is a UI convention rather than a property, and attendees must not be told otherwise.
#
# submitted_at is coarsened to the hour for anonymous forms. Precise timestamps re-identify a
# respondent by correlating against the dispatch ledger.
table "event_feedback_responses" {
  schema  = schema.application
  comment = "feedback answers; registration_id stays NULL on anonymous forms, everywhere"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "form_id" {
    null = false
    type = uuid
  }
  column "registration_id" {
    null    = true
    type    = uuid
    comment = "NULL when the form is anonymous. Never populate it 'just for admins'."
  }
  column "response_data" {
    null    = true
    type    = jsonb
    default = "{}"
    comment = "Answers keyed by field id. Free-form by design."
  }
  column "submitted_at" {
    null    = false
    type    = timestamptz
    default = sql("CURRENT_TIMESTAMP")
    comment = "Coarsen to the hour before writing when the form is anonymous."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_event_feedback_responses_form_id" {
    columns = [column.form_id]
  }

  foreign_key "event_feedback_responses_form_id_fkey" {
    columns     = [column.form_id]
    ref_columns = [table.event_feedback_forms.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "event_feedback_responses_registration_id_fkey" {
    columns     = [column.registration_id]
    ref_columns = [table.registrations.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }
}

# ── Phase 2: networking, growth, analytics ───────────────────────────────────

enum "sync_status" {
  schema = schema.application
  values = ["pending", "synced", "failed", "skipped"]
}

# ── Counter-proposed meeting slots ───────────────────────────────────────────
# The existing `meetings` table holds one slot. FR-9.6 lets a recipient propose a different time,
# which can go back and forth. Storing the counter-proposal on `meetings` would destroy the
# original request; this keeps the negotiation history.
#
# `meeting_status` gained 'slot_proposed' in prompt 02. Accepting a proposal copies its times onto
# the meeting row and sets status back to 'accepted'.
table "meeting_slot_proposals" {
  schema  = schema.application
  comment = "counter-proposed times on a meeting request; preserves the negotiation history"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "meeting_id" {
    null = false
    type = uuid
  }
  column "proposed_by_user_id" {
    null = false
    type = uuid
  }
  column "slot_start" {
    null = false
    type = timestamptz
  }
  column "slot_end" {
    null = false
    type = timestamptz
  }
  column "message" {
    null = true
    type = text
  }
  column "accepted_at" {
    null = true
    type = timestamptz
  }
  column "declined_at" {
    null = true
    type = timestamptz
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_meeting_slot_proposals_meeting_created_at" {
    columns = [column.meeting_id, column.created_at]
  }

  foreign_key "meeting_slot_proposals_meeting_id_fkey" {
    columns     = [column.meeting_id]
    ref_columns = [table.meetings.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "meeting_slot_proposals_proposed_by_user_id_fkey" {
    columns     = [column.proposed_by_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

# ── Reserved slugs ───────────────────────────────────────────────────────────
# events.slug and week_banners.slug are globally unique, and both are checked by a live
# availability lookup. Some slugs must never be claimable because they collide with app routes:
# discover, login, signup, account, dashboard, api, admin, e, w, and so on.
# Seed this table rather than hard-coding an array in the frontend.
table "reserved_slugs" {
  schema  = schema.application
  comment = "slugs no host may claim; seeded, not hard-coded in the client"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "slug" {
    null = false
    type = text
  }
  column "reason" {
    null = true
    type = text
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "reserved_slugs_slug_key" {
    columns = [column.slug]
    unique  = true
  }
}

# ── Referral attribution ─────────────────────────────────────────────────────
# A personal link per person per event. Attribution is recorded at registration time and never
# recomputed - a later click must not rewrite who got credit for an earlier signup.
table "referral_links" {
  schema  = schema.application
  comment = "personal share links for an event, with click and conversion counters"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = false
    type = uuid
  }
  column "owner_user_id" {
    null = false
    type = uuid
  }
  column "code" {
    null    = false
    type    = text
    comment = "The short code appended to the public event URL."
  }
  column "label" {
    null = true
    type = text
  }
  column "click_count" {
    null    = false
    type    = int
    default = 0
  }
  column "registration_count" {
    null    = false
    type    = int
    default = 0
    comment = "Denormalised counter. The authoritative count is COUNT(referral_attributions)."
  }
  column "is_active" {
    null    = false
    type    = boolean
    default = true
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "referral_links_code_key" {
    columns = [column.code]
    unique  = true
  }

  index "idx_referral_links_event_owner" {
    columns = [column.event_id, column.owner_user_id]
  }

  foreign_key "referral_links_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "referral_links_owner_user_id_fkey" {
    columns     = [column.owner_user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

table "referral_attributions" {
  schema  = schema.application
  comment = "immutable record of which referral link produced a registration"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "referral_link_id" {
    null = false
    type = uuid
  }
  column "registration_id" {
    null = false
    type = uuid
  }
  column "attributed_at" {
    null    = false
    type    = timestamptz
    default = sql("CURRENT_TIMESTAMP")
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "referral_attributions_registration_id_key" {
    columns = [column.registration_id]
    unique  = true
    comment = "One registration is attributed to at most one link. First touch wins."
  }

  index "idx_referral_attributions_link_id" {
    columns = [column.referral_link_id]
  }

  foreign_key "referral_attributions_referral_link_id_fkey" {
    columns     = [column.referral_link_id]
    ref_columns = [table.referral_links.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "referral_attributions_registration_id_fkey" {
    columns     = [column.registration_id]
    ref_columns = [table.registrations.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── Analytics substrate ──────────────────────────────────────────────────────
# The final analytics metric set is still an open question with the client, and the behavioural
# journey triggers (viewed / started / completed) need somewhere to land. One generic append-only
# table means every future metric is a query, not a migration.
#
# `activity_type` is TEXT rather than an enum on purpose - the vocabulary will grow weekly and an
# enum append is a migration every time.
#
# Never write personal data into `metadata`. This table feeds aggregates.
table "event_activity_log" {
  schema  = schema.application
  comment = "append-only activity stream feeding event and week analytics; activity_type is open TEXT by design"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "event_id" {
    null = true
    type = uuid
  }
  column "week_id" {
    null    = true
    type    = uuid
    comment = "Set directly for week-page activity; week-level aggregates otherwise roll up through events."
  }
  column "registration_id" {
    null = true
    type = uuid
  }
  column "actor_user_id" {
    null    = true
    type    = uuid
    comment = "NULL for anonymous public-page activity."
  }
  column "activity_type" {
    null    = false
    type    = text
    comment = "page.viewed | registration.started | registration.completed | share.clicked | checkin.completed | material.downloaded"
  }
  column "metadata" {
    null    = true
    type    = jsonb
    default = "{}"
    comment = "Aggregate-friendly context only - referrer, device class, and so on. No personal data."
  }
  column "occurred_at" {
    null    = false
    type    = timestamptz
    default = sql("CURRENT_TIMESTAMP")
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_event_activity_log_event_occurred_at" {
    columns = [column.event_id, column.occurred_at]
  }

  index "idx_event_activity_log_type_occurred_at" {
    columns = [column.activity_type, column.occurred_at]
  }

  foreign_key "event_activity_log_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "event_activity_log_week_id_fkey" {
    columns     = [column.week_id]
    ref_columns = [table.week_banners.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "event_activity_log_registration_id_fkey" {
    columns     = [column.registration_id]
    ref_columns = [table.registrations.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }
}

# ── Outbound integration state ───────────────────────────────────────────────
# Whether and what syncs to the client's Salesforce org is still unresolved, but the sync itself
# will be a Nimbus-side plugin pushing outward. This gives that plugin somewhere to record what it
# has already sent, so it can be built later without a schema change.
#
# entity_id carries no foreign key: it points at rows in many different tables.
table "integration_sync_state" {
  schema  = schema.application
  comment = "per-record outbound sync ledger for external systems; entity_id is polymorphic, no FK"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "system" {
    null    = false
    type    = text
    comment = "salesforce | zoho_analytics | other"
  }
  column "entity_type" {
    null    = false
    type    = text
    comment = "The application table name, e.g. 'events', 'registrations'."
  }
  column "entity_id" {
    null    = false
    type    = uuid
    comment = "Polymorphic. Intentionally not a foreign key."
  }
  column "external_id" {
    null = true
    type = text
  }
  column "status" {
    null    = false
    type    = enum.sync_status
    default = "pending"
  }
  column "sync_hash" {
    null    = true
    type    = text
    comment = "Hash of the payload last sent. Lets the plugin skip records that have not changed."
  }
  column "last_synced_at" {
    null = true
    type = timestamptz
  }
  column "last_error" {
    null = true
    type = text
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "integration_sync_state_entity_system_key" {
    columns = [column.entity_id, column.system]
    unique  = true
  }

  index "idx_integration_sync_state_system_status" {
    columns = [column.system, column.status]
  }
}

# ═════════════════════════════════════════════════════════════════════════════
# AI MATCHMAKING - STUB TABLES ONLY
#
# Structure only, so the eventual implementation is an application change with no migration.
# Nothing reads or writes these yet. Do not add resolvers, GraphQL operations or RBAC grants
# until the matchmaking work is formally scoped.
#
# The scoring model's INPUT fields are real and live elsewhere:
#   climate domain / industry / supply-chain stage / SDG  → taxonomy_terms (+ parent_id, ordinal)
#   what a profile IS vs what it is LOOKING FOR           → profile_taxonomy_terms.is_focus
#   HQ vs stated focus geography                          → profile_geographies.scope
#   fundraising toggle, ask, ticket range                 → profiles
#   seeker-side non-capital support need                  → registrations.expected_outcome
#   opt-in                                                → consents (consent_type='matchmaking')
# ═════════════════════════════════════════════════════════════════════════════

# STUB. Versioned dimension weights so a reweighting is data, not a deploy.
table "matchmaking_weights" {
  schema  = schema.application
  comment = "STUB - versioned scoring dimension weights. Unused; no logic reads this yet."

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "version" {
    null = false
    type = text
  }
  column "dimension_key" {
    null    = false
    type    = text
    comment = "climate_domain | industry | supply_chain_position | geography | stage | ticket_size | non_capital_support | stakeholder_prior | sdg"
  }
  column "weight" {
    null    = false
    type    = decimal(5,4)
    comment = "Weights within a version are expected to sum to 1.0000. Not enforced here."
  }
  column "filter_type" {
    null    = true
    type    = text
    comment = "hard | soft. A hard dimension excludes non-matches rather than lowering their score."
  }
  column "notes" {
    null = true
    type = text
  }
  column "is_active" {
    null    = false
    type    = boolean
    default = false
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "matchmaking_weights_version_dimension_key" {
    columns = [column.version, column.dimension_key]
    unique  = true
  }
}

# STUB. The stakeholder pair matrix - a baseline prior for "does type X typically want to meet
# type Y". Deliberately left EMPTY: the source matrix has not been supplied by the client. Do not
# invent values.
table "matchmaking_stakeholder_priors" {
  schema  = schema.application
  comment = "STUB - stakeholder pair prior matrix. Intentionally unseeded; the source matrix is outstanding."

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "seeker_term_id" {
    null    = false
    type    = uuid
    comment = "taxonomy_terms row with kind='stakeholder_type'."
  }
  column "candidate_term_id" {
    null = false
    type = uuid
  }
  column "prior" {
    null    = false
    type    = decimal(5,4)
    comment = "Baseline affinity. Acts as a floor/ceiling under the other dimensions."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "matchmaking_priors_seeker_candidate_key" {
    columns = [column.seeker_term_id, column.candidate_term_id]
    unique  = true
  }

  foreign_key "matchmaking_stakeholder_priors_seeker_term_id_fkey" {
    columns     = [column.seeker_term_id]
    ref_columns = [table.taxonomy_terms.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "matchmaking_stakeholder_priors_candidate_term_id_fkey" {
    columns     = [column.candidate_term_id]
    ref_columns = [table.taxonomy_terms.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# STUB. Computed match scores, scoped to one event or a whole week.
# Only surfaced to profiles that have opted in - consents.consent_type = 'matchmaking'.
table "matchmaking_scores" {
  schema  = schema.application
  comment = "STUB - computed pairwise match scores. Unused; no scoring job exists yet."

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "seeker_profile_id" {
    null = false
    type = uuid
  }
  column "candidate_profile_id" {
    null = false
    type = uuid
  }
  column "scope" {
    null    = false
    type    = enum.ticket_scope
    default = "event"
    comment = "Reuses the event/week scope vocabulary."
  }
  column "scope_id" {
    null    = true
    type    = uuid
    comment = "The event or week the score was computed within. Polymorphic, no FK."
  }
  column "score" {
    null = false
    type = decimal(6,4)
  }
  column "dimension_scores" {
    null    = true
    type    = jsonb
    default = "{}"
    comment = "Per-dimension breakdown for explainability. Free-form by design."
  }
  column "weights_version" {
    null = true
    type = text
  }
  column "computed_at" {
    null    = false
    type    = timestamptz
    default = sql("CURRENT_TIMESTAMP")
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_matchmaking_scores_seeker_score" {
    columns = [column.seeker_profile_id, column.score]
  }

  index "idx_matchmaking_scores_scope_id" {
    columns = [column.scope_id]
  }

  foreign_key "matchmaking_scores_seeker_profile_id_fkey" {
    columns     = [column.seeker_profile_id]
    ref_columns = [table.profiles.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }

  foreign_key "matchmaking_scores_candidate_profile_id_fkey" {
    columns     = [column.candidate_profile_id]
    ref_columns = [table.profiles.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}

# ── Notification dispatch ledger (class 1 / 2 / 3) ───────────────────────────
# Exactly-once-ish send log. Class 1/2 uniqueness is (registration_id, email_kind).
# Claim protocol: SELECT … FOR UPDATE SKIP LOCKED, then status='sending'.
# Nimbus-owned tables are not altered. GraphQL ops are intentionally omitted -
# Next.js talks to this table over SQL.
table "notification_dispatches" {
  schema  = schema.application
  comment = "per-recipient send ledger for transactional, scheduled, and blast mail"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "class" {
    null    = false
    type    = text
    comment = "transactional | scheduled | blast"
  }
  column "email_kind" {
    null = false
    type = text
  }
  column "registration_id" {
    null = true
    type = uuid
  }
  column "event_id" {
    null = true
    type = uuid
  }
  column "communication_id" {
    null    = true
    type    = uuid
    comment = "Class 3 only."
  }
  column "user_id" {
    null = true
    type = uuid
  }
  column "recipient_email" {
    null = false
    type = text
  }
  column "scheduled_for" {
    null = false
    type = timestamptz
  }
  column "claimed_at" {
    null = true
    type = timestamptz
  }
  column "sent_at" {
    null = true
    type = timestamptz
  }
  column "provider" {
    null    = false
    type    = text
    default = "nimbus_ses"
    comment = "nimbus_ses | salesforce_mc"
  }
  column "provider_message_id" {
    null = true
    type = text
  }
  column "status" {
    null    = false
    type    = text
    default = "pending"
    comment = "pending | sending | sent | failed | skipped | cancelled"
  }
  column "attempt_count" {
    null    = false
    type    = int
    default = 0
  }
  column "max_attempts" {
    null    = false
    type    = int
    default = 3
  }
  column "last_error" {
    null = true
    type = text
  }
  column "next_attempt_at" {
    null = true
    type = timestamptz
  }
  column "idempotency_key" {
    null = false
    type = text
  }
  column "created_at" {
    null    = false
    type    = timestamptz
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = true
    type    = timestamptz
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "notification_dispatches_reg_kind_uniq" {
    unique  = true
    columns = [column.registration_id, column.email_kind]
    where   = "class IN ('transactional', 'scheduled')"
  }

  index "notification_dispatches_blast_uniq" {
    unique  = true
    columns = [column.communication_id, column.registration_id]
    where   = "class = 'blast'"
  }

  index "idx_notification_dispatches_claim" {
    columns = [column.status, column.next_attempt_at]
  }

  index "idx_notification_dispatches_event_id" {
    columns = [column.event_id]
  }

  index "idx_notification_dispatches_idempotency_key" {
    unique  = true
    columns = [column.idempotency_key]
  }

  check "notification_dispatches_class_chk" {
    expr = "class IN ('transactional', 'scheduled', 'blast')"
  }

  check "notification_dispatches_status_chk" {
    expr = "status IN ('pending', 'sending', 'sent', 'failed', 'skipped', 'cancelled')"
  }

  foreign_key "notification_dispatches_registration_id_fkey" {
    columns     = [column.registration_id]
    ref_columns = [table.registrations.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  foreign_key "notification_dispatches_event_id_fkey" {
    columns     = [column.event_id]
    ref_columns = [table.events.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  foreign_key "notification_dispatches_communication_id_fkey" {
    columns     = [column.communication_id]
    ref_columns = [table.communications.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }

  foreign_key "notification_dispatches_user_id_fkey" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = SET_NULL
  }
}

# ── Media object registry (filesystem analogue) ─────────────────────────────
#
# Application-owned inventory of every object written to the public bucket.
# Bytes live in the object store; this table is the durable record of each object
# (name, size, type, key, owner, purpose, lifecycle). Domain attachments
# (parent event/week/session links) land later.
#
# owned_by is the session user (application.users.id). Do not invent parallel
# user tables. Principle 20: do not recreate Nimbus file_object here.
enum "media_purpose" {
  schema = schema.application
  values = [
    "cover",
    "banner",
    "logo",
    "speaker_photo",
    "session_material",
    "registration_import",
    "avatar",
    "other"
  ]
}

enum "media_visibility" {
  schema = schema.application
  values = ["public_read", "authenticated", "private"]
}

enum "media_status" {
  schema = schema.application
  values = ["active", "soft_deleted", "orphan"]
}

table "media_objects" {
  schema  = schema.application
  comment = "filesystem-style registry of every object saved to the public bucket"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "object_key" {
    null = false
    type = text
    comment = "Bucket object key (unique). Same shape Next.js mints for PUT."
  }
  column "file_name" {
    null = false
    type = text
  }
  column "file_size" {
    null = false
    type = bigint
  }
  column "file_type" {
    null = false
    type = text
  }
  column "file_extension" {
    null    = true
    type    = text
  }
  column "content_sha256" {
    null = true
    type = text
  }
  column "storage_bucket" {
    null    = true
    type    = text
    comment = "Bucket name used for the object (PUBLIC_CONTENT_BUCKET / ARTIFACT_DATA_BUCKET)."
  }
  column "visibility" {
    null    = false
    type    = enum.media_visibility
    default = "public_read"
  }
  column "purpose" {
    null    = false
    type    = enum.media_purpose
    default = "other"
  }
  column "status" {
    null    = false
    type    = enum.media_status
    default = "active"
  }
  column "owned_by" {
    null = false
    type = uuid
    comment = "Session user id (application.users.id) that created / owns this object."
  }
  column "deleted_at" {
    null    = true
    type    = timestamptz
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  column "metadata" {
    null = true
    type = jsonb
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_media_objects_object_key" {
    columns = [column.object_key]
    unique  = true
  }

  index "idx_media_objects_owned_by" {
    columns = [column.owned_by]
  }

  index "idx_media_objects_status" {
    columns = [column.status]
  }

  foreign_key "fk_media_objects_owned_by" {
    columns     = [column.owned_by]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

# Vendored from upstream fork schema-application.hcl (dev @ cc9ed5c, "fixes #5":
# provider token-exchange endpoint). Nimbus-owned. Do not ALTER except to keep this table
# in sync with the pin.
table "user_identity" {
  schema  = schema.application
  comment = "Maps a provider identity to a Nimbus user"

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }

  column "user_id" {
    type = uuid
    null = false
    comment = "The application.users row this identity belongs to"
  }

  column "provider" {
    type = text
    null = false
    comment = "Provider name, e.g. google, github"
  }

  column "provider_user_id" {
    type = text
    null = false
    comment = "Stable user id within the provider"
  }

  column "email" {
    type = text
    null = true
  }

  column "created_at" {
    type    = timestamp
    null    = false
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_user_identity_provider" {
    columns = [column.provider, column.provider_user_id]
    unique  = true
  }

  foreign_key "fk_user_identity_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
}
