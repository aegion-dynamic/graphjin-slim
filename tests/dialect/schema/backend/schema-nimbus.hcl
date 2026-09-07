schema "accesscontrol" {
    comment = " Access control schema "
}

table "teams" {
  schema = schema.accesscontrol
  column "id" {
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "name" {
    type = text
    null = false
  }
  column "permissions" {
    type = json
    null = false
  }

  primary_key {
    columns = [column.id]
  }
}

table "endpointaccess"  {
  schema = schema.accesscontrol
  comment = "Table for storing endpoint access control"

  column "id" {
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "role"{
    type = text
    null = false
  }
  column "endpoint"{
    type = text
    null = false
  }
  column "viewer"{
    type =  boolean
    null = true
  }
  column "contributor"{
    type = boolean
    null = true
  }
  column "admin"{
    type = boolean
    null = true
  }

  column "origin" {
    type    = text
    null    = false
    default = "seed"
    comment = "Provenance: 'seed' rows are reconciled to useraccess.json by nimbus-migrate (removed-from-JSON => deleted); 'runtime' rows are added via the REST APIs and are never touched by the seeder."
  }

  column "created_time"{
    type    = timestamp
    null    = false
    default = sql("CURRENT_TIMESTAMP")
  }
  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_role_endpointaccess" {
    columns     = [column.role]
    ref_columns = [table.role.column.rolename]
}

  index "role_endpoint"{
    columns = [
      column.role,
      column.endpoint
    ]
    unique = true
  }

}



table "role" {
  schema = schema.accesscontrol
  comment = "Table for storing roles in the system"

 column "id" {
    type = uuid
    default = sql("gen_random_uuid()")
    null = false
  }
 
  column "rolename"{
    type = text
    null = false

  }

  column "parent_role_id" {
    type    = uuid
    null    = true
    comment = "Self-reference: this role inherits all permissions of its parent (and ancestors)."
  }

  column "is_builtin" {
    type    = boolean
    null    = false
    default = false
    comment = "True for seeded/predefined roles; false for org-owned custom roles."
  }

  column "is_default" {
    type    = boolean
    null    = false
    default = false
    comment = "True for the single global role auto-attached to newly provisioned users. Seeded from useraccess.json default:true."
  }

  column "organization_id" {
    type    = uuid
    null    = true
    comment = "Owning organization for a custom org-scoped role. NULL = global/built-in role."
  }

  column "description" {
    type = text
    null = true
  }

  column "created_at" {
    type    = timestamp
    null    = false
    default = sql("CURRENT_TIMESTAMP")
  }

  column "updated_at" {
    type    = timestamp
    null    = false
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }

  // Self-FK for the role hierarchy (a role inherits its parent's permissions).
  foreign_key "fk_role_parent" {
    columns     = [column.parent_role_id]
    ref_columns = [table.role.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  // Cross-schema FK: a custom role is owned by an application.organization.
  foreign_key "fk_role_organization" {
    columns     = [column.organization_id]
    ref_columns = [table.organization.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  // rolename stays globally unique: endpointaccess/operation_access/user_role_mapping
  // still hold role.rolename FKs, which require a non-partial unique. Per-org name reuse
  // is a documented follow-on (migrate those FKs to role.id first). See docs/RBAC_DESIGN.md.
  index "role_index" {
    columns = [
      column.rolename
    ]
    unique = true
  }
}

table "user_role_mapping"{
  schema = schema.accesscontrol
  comment = "Table for storing user role mapping"

  column "id"{
    type = uuid
    default = sql("gen_random_uuid()")
    null = false
  }

  column "user_id"{
    type = uuid
    null = false
  }
  column "role_name"{
    type = text
    null = false
  }
  column "role_id"{
    type    = uuid
    null    = true
    comment = "FK to role.id; backfilled from role_name by nimbus-migrate. Used by the effective-permission view for hierarchy joins; role_name kept for backward compatibility."
  }
  column "is_default"{
    type    = boolean
    null    = false
    default = false
    comment = "True when this mapping was auto-attached as the default role at user provisioning. False for admin or API grants."
  }
  column "created_time"{
    type    = timestamp
    null    = false
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_time"{
    type    = timestamp
    null    = true
  }
  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_userid_user_mapping" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
  }
  foreign_key "fk_role_user_mapping"{
    columns     = [column.role_name]
    ref_columns = [table.role.column.rolename]
  }
  foreign_key "fk_roleid_user_mapping"{
    columns     = [column.role_id]
    ref_columns = [table.role.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }

  index "user_role"{
    columns=[ 
      column.user_id,
      column.role_name
    ]
    unique = true
  }
}

table "operation_access"{
  schema = schema.accesscontrol
  comment = "Table for storing operation access control"

  column "id"{
    type = uuid
    default = sql("gen_random_uuid()")
    null = false
  }
  column "operation"{
    type = text
    null = false
  }
  column "role"{
    type = text
    null = false
  }
  column "origin" {
    type    = text
    null    = false
    default = "seed"
    comment = "Provenance: 'seed' rows are reconciled to useraccess.json by nimbus-migrate; 'runtime' rows (added via the REST APIs) are never touched by the seeder."
  }
  column "created_at"{
    type    = timestamp
    null    = false
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at"{
    type    = timestamp
    null    = true
  }

  primary_key {
    columns = [column.id]
  }
  foreign_key "fk_role_operation_access"{
    columns     = [column.role]
    ref_columns = [table.role.column.rolename]
  }

  // Index for role-based lookups
  index "idx_operation_access_role" {
      columns = [column.role]
  }

  // Composite index for role+operation lookups
  index "idx_operation_access_role_operation" {
      columns = [column.role, column.operation]
      unique = true
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// RBAC engine (P2): permission catalog + role→permission map + additive grant
// sources (org base role, team role). Role hierarchy lives on role.parent_role_id.
// The effective-permission derivation is a VIEW created by nimbus-migrate (Atlas
// gates view blocks behind `atlas login`), so it is intentionally NOT declared here.
// See docs/RBAC_DESIGN.md.
// ─────────────────────────────────────────────────────────────────────────────

// The three gating surfaces a permission can apply to, plus the legacy REST endpoint.
enum "permission_kind_enum" {
  schema = schema.accesscontrol
  values = ["operation", "page", "action", "endpoint"]
}

// permission is the capability catalog. `key` is a stable capability string, e.g.
// "CreatePurchaseOrder" (operation), "/dashboard" (page), "user.create" (action).
table "permission" {
  schema  = schema.accesscontrol
  comment = "Capability catalog; permissions gate operations, pages, actions, and endpoints."

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "key" {
    type    = text
    null    = false
    comment = "Stable capability key, unique across kinds."
  }
  column "kind" {
    type = enum.permission_kind_enum
    null = false
  }
  column "description" {
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
  index "idx_permission_key_unique" {
    columns = [column.key]
    unique  = true
  }
  index "idx_permission_kind" {
    columns = [column.kind]
  }
}

// role_permission maps a role to the permissions it directly grants. A role's
// EFFECTIVE permissions are its own rows here plus those of all ancestor roles
// (role.parent_role_id), computed by the effective-permission view.
table "role_permission" {
  schema  = schema.accesscontrol
  comment = "Role → permission grants. Hierarchy expansion happens in the derivation view."

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "role_id" {
    type = uuid
    null = false
  }
  column "permission_id" {
    type = uuid
    null = false
  }
  column "created_at" {
    type    = timestamp
    null    = false
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }
  foreign_key "fk_role_permission_role" {
    columns     = [column.role_id]
    ref_columns = [table.role.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  foreign_key "fk_role_permission_permission" {
    columns     = [column.permission_id]
    ref_columns = [table.permission.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  index "idx_role_permission_unique" {
    columns = [column.role_id, column.permission_id]
    unique  = true
  }
  index "idx_role_permission_role" {
    columns = [column.role_id]
  }
}

// organization_role is the org base role: every member of the organization
// (application.organization_membership) additively gets this role's permissions.
table "organization_role" {
  schema  = schema.accesscontrol
  comment = "Org base role: granted additively to every member of the organization."

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "organization_id" {
    type = uuid
    null = false
  }
  column "role_id" {
    type = uuid
    null = false
  }
  column "created_at" {
    type    = timestamp
    null    = false
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }
  // Cross-schema FK to application.organization (bare table name convention).
  foreign_key "fk_organization_role_org" {
    columns     = [column.organization_id]
    ref_columns = [table.organization.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  foreign_key "fk_organization_role_role" {
    columns     = [column.role_id]
    ref_columns = [table.role.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  index "idx_organization_role_unique" {
    columns = [column.organization_id, column.role_id]
    unique  = true
  }
}
// team_role grants a role additively to every member of a team
// (application.team_membership), including members of child teams via
// team.parent_team_id (members of a child team additively inherit parent teams roles).
table "team_role" {
  schema  = schema.accesscontrol
  comment = "Team role: granted additively to every member of the team (and to members of child teams through the hierarchy)."

  column "id" {
    type    = uuid
    null    = false
    default = sql("gen_random_uuid()")
  }
  column "team_id" {
    type = uuid
    null = false
  }
  column "role_id" {
    type = uuid
    null = false
  }
  column "created_at" {
    type    = timestamp
    null    = false
    default = sql("CURRENT_TIMESTAMP")
  }

  primary_key {
    columns = [column.id]
  }
  // Cross-schema FK to application.team (bare table name convention).
  foreign_key "fk_team_role_team" {
    columns     = [column.team_id]
    ref_columns = [table.team.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  foreign_key "fk_team_role_role" {
    columns     = [column.role_id]
    ref_columns = [table.role.column.id]
    on_update   = NO_ACTION
    on_delete   = CASCADE
  }
  index "idx_team_role_unique" {
    columns = [column.team_id, column.role_id]
    unique  = true
  }
}

// role_assignment_policy declares which roles (or org-admin) may attach/detach which target roles.
// Seeded from useraccess.json canAttach/canDetach (+ organizationAdmin). Runtime checks read this table.
table "role_assignment_policy" {
  schema  = schema.accesscontrol
  comment = "Which roles (or org-admin) may attach/detach which target roles"

  column "id" {
    type    = uuid
    default = sql("gen_random_uuid()")
    null    = false
  }
  column "assigner_role" {
    type = text
    null = false
    comment = "Global role name, or __organization_admin__ for org-admin matrix"
  }
  column "target_role" {
    type = text
    null = false
  }
  column "can_attach" {
    type    = boolean
    null    = false
    default = false
  }
  column "can_detach" {
    type    = boolean
    null    = false
    default = false
  }
  column "created_at" {
    type    = timestamp
    null    = false
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    type = timestamp
    null = true
  }

  primary_key {
    columns = [column.id]
  }

  index "idx_role_assignment_policy_assigner_target" {
    columns = [column.assigner_role, column.target_role]
    unique  = true
  }

  index "idx_role_assignment_policy_assigner" {
    columns = [column.assigner_role]
  }
}


schema "core" {
  comment = " standard nimbus schema "
}

table "file_object" {
    schema = schema.core
    comment = "Table for capturing operational data of file storage"

    // Unique ID for the file object inside the RDBS
    column "id" {
        type = uuid
        null = false
        default = sql("gen_random_uuid()")
    }

    // Name of the file (as uploaded by the user)
    column "file_name" {
        type = text
        null = false
    }

    // URI of the file in the storage system
    column "file_uri" {
        type = text
        null = true
    }

    // Size of the file in bytes
    column "file_size" {
        type = bigint
        null = false
    }

    // TODO - Create a file type enum
    column "file_type" {
      type = text
      null = false
    }

    // File Type of the file
    column "file_extension" {
        type = text
        null = true
    }

    // Timestamp when the file was created
    column "created_at" {
        type = timestamp
        null = false
        default = sql("CURRENT_TIMESTAMP")
    }

    // Timestamp when the file was last updated
    column "updated_at" {
        type = timestamp
        null = true
        default = sql("CURRENT_TIMESTAMP")
    }

    // JSONB column to store metadata about the file
    column "metadata" {
        type = jsonb
        null = true
    }

    primary_key{
        columns = [column.id]
    }

    index "idx_file_name" {
        columns = [column.file_name]
        unique = true
    }

}

table "queues" {
    schema = schema.core
    comment = "Table for capturing operational data of task data"

    column "id" {
        type = uuid
        null = false
        default = sql("gen_random_uuid()")
    }

    column "queue_name" {
        type = text
        null = false
    }

    column "created_at" {
        type = timestamp
        null = false
        default = sql("CURRENT_TIMESTAMP")
    }

    primary_key{
        columns = [column.id]
    }

    index "idx_queue_name" {
        columns = [column.queue_name]
        unique = true
    }

}

enum "task_status_enum" {
    schema = schema.core
    values = ["TO_DO", "IN_PROGRESS", "DONE"]
}

table "tasks" {
    schema = schema.core
    comment = "Table for capturing operational data of task queue"

    column "id" {
        type = uuid
        null = false
        default = sql("gen_random_uuid()")
    }

    column "status" {
        type = enum.task_status_enum
        null = false
    }

    column "created_at" {
        type = timestamp
        null = false
        default = sql("CURRENT_TIMESTAMP")
    }

    column "started_at" {
        type = timestamp
        null = true
    }

    column "completed_at" {
        type = timestamp
        null = true
    }

    column "payload" {
        // TODO- Change this to type = jsonb
        type = text
        null = true
    }

    column "queue_id" {
        type = uuid
        null = false
    }

    primary_key{
        columns = [column.id]
    }

    foreign_key "fk_task_queue_tasks_id" {
      columns = [column.queue_id]
      ref_columns = [table.queues.column.id]
    }

    // Index for status-based filtering (used in PopTaskFromQueue)
    index "idx_tasks_status" {
        columns = [column.status]
    }

    // Composite index for efficient queue task lookups (PopTaskFromQueue, GetAllTasksFromQueue)
    index "idx_tasks_queue_status_created" {
        columns = [column.queue_id, column.status, column.created_at]
    }
}

table "cache" {
    schema = schema.core
    comment = "Table for storing the caches in the database"

    column "id" {
        type = uuid
        null = false
        default = sql("gen_random_uuid()")
    }

    column "memo" {
        type = text
        null = false
    }

    column "payload" {
        type = text
        null = false
    }

    column "startdate" {
        type = timestamp
        null = false
        
    }

    column "enddate" {
        type = timestamp
        null = false
    }

    column "lookupkey" {
        type = text
        null = true
    }

     primary_key{
        columns = [column.id]
    }

    index "cache_index" {
    columns = [
      column.memo,
      column.lookupkey
    ]
    unique = true
  }


}

table "file_serve" {
    schema = schema.core
    comment = "Table for storing sharable file links for external users"

    column "id" {
        type = uuid
        null = false
        default = sql("gen_random_uuid()")
    }

    column "file_id" {
        type = uuid
        null = false
    }

    column "created_by_user_id" {
        type = uuid
        null = false
    }

    column "password" {
        type = text
        null = true
    }

    column "max_downloads" {
        type = integer
        null = false
        default = -1
    }

    column "downloads_count" {
        type = integer
        null = false
        default = 0
    }

    column "expiry" {
        type = timestamp
        null = true
    }

    column "access_past_versions" {
        type = boolean
        null = false
        default = false
    }

    column "created_at" {
        type = timestamp
        null = false
        default = sql("CURRENT_TIMESTAMP")
    }

    column "updated_at" {
        type = timestamp
        null = false
        default = sql("CURRENT_TIMESTAMP")
    }

    primary_key {
        columns = [column.id]
    }

    foreign_key "fk_file_serve_file" {
        columns = [column.file_id]
        ref_columns = [table.file_object.column.id]
        on_update = NO_ACTION
        on_delete = NO_ACTION
    }

    foreign_key "fk_file_serve_user" {
        columns = [column.created_by_user_id]
        ref_columns = [table.users.column.id]
        on_update = NO_ACTION
        on_delete = NO_ACTION
    }
}

table "sequences" {
    schema  = schema.core
    comment = "Table for managing named auto-increment sequences"

    column "id" {
        type    = uuid
        null    = false
        default = sql("gen_random_uuid()")
    }

    column "organization_id" {
        type = uuid
        null = false
    }

    column "lookup_key" {
        type = text
        null = false
    }

    column "format_string" {
        type    = text
        null    = false
        default = "{lookup_key}-{year}-{value}"
        comment = "Format template with placeholders: {lookup_key}, {value} (zero-padded to min_digits), {year}, {min_digits}. Validated on create."
    }

    column "label" {
        type = text
        null = true
    }

    column "description" {
        type = text
        null = true
    }

    column "current_value" {
        type    = bigint
        null    = false
        default = 0
    }

    column "min_digits" {
        type    = integer
        null    = false
        default = 5
    }

    column "include_year" {
        type    = boolean
        null    = false
        default = false
    }

    column "created_at" {
        type    = timestamp
        null    = false
        default = sql("CURRENT_TIMESTAMP")
    }

    column "updated_at" {
        type    = timestamp
        null    = false
        default = sql("CURRENT_TIMESTAMP")
    }

    primary_key {
        columns = [column.id]
    }

    // Composite unique: same lookup_key allowed across different organizations
    index "idx_sequence_org_lookup_key" {
        columns = [column.organization_id, column.lookup_key]
        unique  = true
    }

    foreign_key "fk_sequence_organization" {
        columns     = [column.organization_id]
        ref_columns = [table.organization.column.id]
        on_update   = NO_ACTION
        on_delete   = NO_ACTION
    }
}

enum "audit_action_enum" {
    schema = schema.core
    values = ["INSERT", "UPDATE", "DELETE"]
}

table "audit_logs" {
    schema  = schema.core
    comment = "Immutable audit trail for all data mutations (SOC 2 / GDPR / ISO 27001)"

    column "id" {
        type    = uuid
        null    = false
        default = sql("gen_random_uuid()")
    }

    column "table_schema" {
        type = text
        null = false
    }

    column "table_name" {
        type = text
        null = false
    }

    // Nullable: audited tables are not required to have an `id` column. Composite-PK
    // tables have no single record identifier, so core.audit_trigger_func writes NULL
    // rather than failing the write (issue #277).
    column "record_id" {
        type = text
        null = true
    }

    column "action" {
        type = enum.audit_action_enum
        null = false
    }

    column "old_data" {
        type = jsonb
        null = true
    }

    column "new_data" {
        type = jsonb
        null = true
    }

    column "changed_fields" {
        type = sql("text[]")
        null = true
    }

    column "user_id" {
        type = text
        null = true
    }

    column "user_email" {
        type = text
        null = true
    }

    column "ip_address" {
        type = text
        null = true
    }

    column "created_at" {
        type    = timestamptz
        null    = false
        default = sql("NOW()")
    }

    primary_key {
        columns = [column.id]
    }

    index "idx_audit_logs_table_record" {
        columns = [column.table_schema, column.table_name, column.record_id]
    }

    index "idx_audit_logs_user_id" {
        columns = [column.user_id]
    }

    index "idx_audit_logs_created_at" {
        columns = [column.created_at]
    }
}



// TODO: all the schemas are now pointed to application, we should however modify 
// this in future once graphjin's multischema support gets solidified
schema "users" {
    comment = "User information and the entire persmissions model"
}


// "authenticated_token_pairs" table to store the token pairs for the user
// This will be used when we need to create oAuth tokens for the user
table "authenticated_token_pairs" {
  schema = schema.application
  
  column "id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "key" {
    null = false
    type = text
  }
  column "secret" {
    null = false
    type = text
  }

  // The token is issued at the time of creation
  column "issued_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }

  // Enables a perpetual token if set to null
  column "expires_at" {
    type = timestamp
    null = true
  }

  column "user_id" {
      type = uuid
      null = false
  }

  primary_key {
    columns = [column.id]
  }

  foreign_key "fk_authenticated_token_pairs_user" {
    ref_columns = [ table.users.column.id ]
    columns = [ column.user_id ]
  }
}

// "users" table stores canonical application users and their profile data.
// -- Pruthvi 16-Dec-23: Need to add phone number for a user data, irrespective of whether or not we use it.
table "users" {
  schema = schema.application
  column "id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "first_name" {
    null = false
    type = text
  }
  column "last_name" {
    null = false
    type = text
  }
  column "email" {
    null = false
    type = text
  }

  column "phone" {
    null = false
    type = text
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

  column "external_id" {
    type = text
    null = false
    comment = "Stable internal subject for social users; legacy provider external IDs remain supported"
  }

  column "avatar_url" {
    type = text
    null = true
  }

  column "is_anonymous" {
    type    = boolean
    null    = false
    default = sql("false")
    comment = "Anonymous visitor minted via POST /auth/anonymous"
  }

  primary_key {
    columns = [column.id]
  }

  index "email_extid"{
    columns = [
      column.external_id,
      column.email
    ]
    unique = true
  }
}

// The table user_data is used to store arbitrary data for a user.
// All the data can be retrieved using the user_id and the look_reference.
table "user_data" {
    schema = schema.application
    column "id" {
        null = false
        type = uuid
        default = sql("gen_random_uuid()")
    }

    column "user_id" {
        type = uuid
        null = false
    }

    column "lookup_refernce" {
        type = text
        null = false
    }

    column data {
        type = text
        null = false
    }

    primary_key  {
      columns = [ column.id ]
    }
    
    index "user_data_index" {
      columns = [ column.lookup_refernce ]
    }
}

// Team visibility (GitHub-style): a "visible" team is listed to all org members; a
// "secret" team is visible only to its own members and the org's admins.
enum "team_visibility_enum" {
  schema = schema.application
  values = ["visible", "secret"]
}

// The table team is used to store the team information / Equivalent to a group
table "team" {
  schema = schema.application
  column "id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "name" {
    null = false
    type = text
  }
  column "organization_id" {
    null = false
    type = uuid
  }
  column "parent_team_id" {
    null    = true
    type    = uuid
    comment = "Self-reference for team nesting. Members of a child team additively inherit parent teams' roles."
  }
  column "visibility" {
    null    = false
    type    = enum.team_visibility_enum
    default = "visible"
    comment = "GitHub-style team visibility: visible (to all org members) or secret (members + org admins only)."
  }
  column "allow_join_requests" {
    null    = false
    type    = boolean
    default = false
    comment = "Frontend-configurable: when true, org members may request to join this team (maintainer approves). Optional feature, off by default."
  }
  column "allow_invitations" {
    null    = false
    type    = boolean
    default = true
    comment = "Frontend-configurable: when true, maintainers may invite users to this team. Optional feature, on by default."
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "fk_team_organization" {
    columns     = [column.organization_id]
    ref_columns = [table.organization.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  foreign_key "fk_team_parent" {
    columns     = [column.parent_team_id]
    ref_columns = [table.team.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  // Team names are unique within an organization (GitHub-style).
  index "idx_team_org_name_unique" {
    columns = [column.organization_id, column.name]
    unique  = true
  }
}

// Table team_admin is used to store the team admin information
table "team_admin" {
  schema = schema.application
  column "id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "team_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null = false
    type = uuid
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "fk_team_admin_team" {
    columns     = [column.team_id]
    ref_columns = [table.team.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  foreign_key "fk_team_admin_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  // A user is a maintainer of a given team at most once.
  index "idx_team_admin_unique" {
    columns = [column.team_id, column.user_id]
    unique  = true
  }
  // The RBAC hot path probes maintainers by user (IsTeamMaintainer); the composite
  // unique index above leads with team_id, so user_id needs its own leading index.
  index "idx_team_admin_user" {
    columns = [column.user_id]
  }
}

// Table team_membership is used to store the team membership information
table "team_membership" {
  schema = schema.application
  column "id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "team_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null = false
    type = uuid
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "fk_team_membership_team" {
    columns     = [column.team_id]
    ref_columns = [table.team.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  foreign_key "fk_team_membership_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  // Prevent duplicate memberships (team membership is now an authorization source).
  index "idx_team_membership_unique" {
    columns = [column.team_id, column.user_id]
    unique  = true
  }
  // The RBAC hot path (effective_user_access team CTE) probes memberships by user;
  // the composite unique index above leads with team_id, so user_id needs its own
  // leading index to avoid a seq scan per request.
  index "idx_team_membership_user" {
    columns = [column.user_id]
  }
}

// Table organization is used to store the organization information
// -- Pruthvi 16-Dec-23: Does this table refer to the organizations that will purchase the software? As in the docs/techs who will use this? 
// -- Pruthvi 16-Dec-23: If so, the company that the patient works for needs to be a different table/field
table "organization" {
  schema = schema.application
  column "id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "name" {
    null = false
    type = text
  }
  column "slug" {
    null    = false
    type    = text
    comment = "Unique URL-friendly identifier for the organization, displayed under the org name"
  }
  column "metadata" {
    null    = true
    type    = jsonb
    default = "{}"
    comment = "Key-value store for organization details (address, phone_number, email, logo_id, etc.)"
  }
  column "owner_id" {
    null    = true
    type    = uuid
    comment = "The organization's owner (distinct from admins): may transfer ownership and delete the org. NULL for legacy orgs."
  }
  column "created_at" {
    null    = false
    type    = timestamp
    default = sql("CURRENT_TIMESTAMP")
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "fk_organization_owner" {
    columns     = [column.owner_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  index "idx_org_slug_unique" {
    columns = [column.slug]
    unique  = true
  }
}

// Table organization_admin is used to store the organization admin information
table "organization_admin" {
  schema = schema.application
  column "id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "organization_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null = false
    type = uuid
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "fk_organization_admin_organization" {
    columns     = [column.organization_id]
    ref_columns = [table.organization.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  foreign_key "fk_organization_admin_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  index "idx_org_admin_unique" {
    columns = [column.organization_id, column.user_id]
    unique  = true
  }
  // The RBAC org probes (IsOrgAdmin/superOrExists) check membership by user; the
  // composite unique index above leads with organization_id, so user_id needs its
  // own leading index.
  index "idx_org_admin_user" {
    columns = [column.user_id]
  }
}

// Table organization_membership is used to store the organization membership information
table "organization_membership" {
  schema = schema.application
  column "id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "organization_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null = false
    type = uuid
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "fk_organization_membership_organization" {
    columns     = [column.organization_id]
    ref_columns = [table.organization.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  foreign_key "fk_organization_membership_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  index "idx_org_membership_unique" {
    columns = [column.organization_id, column.user_id]
    unique  = true
  }
  // The RBAC hot path (effective_user_access org_base CTE) probes memberships by
  // user; the composite unique index above leads with organization_id, so user_id
  // needs its own leading index to avoid a seq scan per request.
  index "idx_org_membership_user" {
    columns = [column.user_id]
  }
}

// Enum for organization access request status
enum "access_request_status_enum" {
  schema = schema.application
  values = ["pending", "approved", "rejected"]
}

// Table organization_access_request stores requests from users wanting to join an organization
table "organization_access_request" {
  schema = schema.application
  column "id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "organization_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "status" {
    null = false
    type = enum.access_request_status_enum
    default = "pending"
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
  primary_key {
    columns = [column.id]
  }
  index "idx_access_request_org_status" {
    columns = [column.organization_id, column.status]
  }
  foreign_key "fk_access_request_organization" {
    columns     = [column.organization_id]
    ref_columns = [table.organization.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  foreign_key "fk_access_request_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

// Enum for organization invitation status (admin-initiated, the inverse of access requests).
enum "organization_invitation_status_enum" {
  schema = schema.application
  values = ["pending", "accepted", "declined", "revoked"]
}

// Table organization_invitation stores admin-issued invitations for a user to join an org.
// This is the inverse direction of organization_access_request (which is user-initiated).
table "organization_invitation" {
  schema = schema.application
  column "id" {
    null    = false
    type    = uuid
    default = sql("gen_random_uuid()")
  }
  column "organization_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null    = false
    type    = uuid
    comment = "The invited user."
  }
  column "invited_by" {
    null    = false
    type    = uuid
    comment = "The org admin who issued the invitation."
  }
  column "status" {
    null    = false
    type    = enum.organization_invitation_status_enum
    default = "pending"
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
  primary_key {
    columns = [column.id]
  }
  index "idx_org_invitation_org_status" {
    columns = [column.organization_id, column.status]
  }
  index "idx_org_invitation_user_status" {
    columns = [column.user_id, column.status]
  }
  foreign_key "fk_org_invitation_organization" {
    columns     = [column.organization_id]
    ref_columns = [table.organization.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  foreign_key "fk_org_invitation_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  foreign_key "fk_org_invitation_invited_by" {
    columns     = [column.invited_by]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

// Team membership requests — OPTIONAL, per-team-configurable. One unified table for both
// directions: a maintainer-issued invitation (kind=invite) and a user-issued join request
// (kind=request). Gated by team.allow_invitations / team.allow_join_requests.
enum "team_membership_request_kind_enum" {
  schema = schema.application
  values = ["invite", "request"]
}

enum "team_membership_request_status_enum" {
  schema = schema.application
  values = ["pending", "accepted", "declined", "revoked"]
}

table "team_membership_request" {
  schema = schema.application
  column "id" {
    null    = false
    type    = uuid
    default = sql("gen_random_uuid()")
  }
  column "team_id" {
    null = false
    type = uuid
  }
  column "user_id" {
    null    = false
    type    = uuid
    comment = "The invited user (invite) or the requesting user (request)."
  }
  column "kind" {
    null = false
    type = enum.team_membership_request_kind_enum
  }
  column "status" {
    null    = false
    type    = enum.team_membership_request_status_enum
    default = "pending"
  }
  column "invited_by" {
    null    = true
    type    = uuid
    comment = "The maintainer who issued an invitation; NULL for a user-initiated join request."
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
  primary_key {
    columns = [column.id]
  }
  // 2-column indexes (kept to 2 cols: bobgen v0.42 orders 3-column index columns
  // non-deterministically, which flakes the drift gate; 2-col indexes are stable).
  index "idx_team_mreq_team_kind" {
    columns = [column.team_id, column.kind]
  }
  index "idx_team_mreq_user_kind" {
    columns = [column.user_id, column.kind]
  }
  foreign_key "fk_team_mreq_team" {
    columns     = [column.team_id]
    ref_columns = [table.team.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  foreign_key "fk_team_mreq_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
  foreign_key "fk_team_mreq_invited_by" {
    columns     = [column.invited_by]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    on_delete   = NO_ACTION
  }
}

// Table user_permissions is used to store the permissions information. This is modeled after the unix file system permissions
table "user_permissions" {
  schema = schema.application

  column "id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "user_id" {
    null = false
    type = uuid
  }
  column "resource_type" {
    null = false
    type = enum.resource_type_enum
  }
  column "resource_id" {
    null = false
    type = uuid
  }
  column "read" {
    null    = false
    type    = boolean
    default = false
  }
  column "write" {
    null    = false
    type    = boolean
    default = false
  }
  column "execute" {
    null    = false
    type    = boolean
    default = false
  }
  column "owner" {
    null    = false
    type    = boolean
    default = false
    comment = "True for the row representing the resource's current owner (full control; may manage grants and transfer ownership). Exactly one per resource once transferred."
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "fk_permissions_user" {
    columns     = [column.user_id]
    ref_columns = [table.users.column.id]
    on_update   = NO_ACTION
    // CASCADE: a deleted user's ACL rows are meaningless and would otherwise block
    // DeleteNimbusUserRecord (the FK previously used NO_ACTION, so deleting any user
    // who had ever been granted an owner ACL failed after Cognito was already gone).
    on_delete   = CASCADE
  }
  // Hot-path lookup: HasResourceACL probes (resource_type, resource_id) on every
  // file read / queue write+delete.
  index "idx_user_permissions_resource" {
    columns = [column.resource_type, column.resource_id]
  }
  // One grant per (user, resource): GrantOwnerACL's check-then-insert is racy under
  // concurrent creates; the constraint makes the race impossible.
  index "uq_user_permissions_user_resource" {
    columns = [column.user_id, column.resource_type, column.resource_id]
    unique  = true
  }
}

// Emum for resource types
enum "resource_type_enum" {
  schema = schema.application
  values = ["file_object", "task_queue"]
}

// Table user_permissions is used to store the permissions information. This is modeled after the unix file system permissions
table "team_permissions" {
  schema = schema.application

  column "id" {
    null = false
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "team_id" {
    null = false
    type = uuid
  }
  column "resource_type" {
    null = false
    type = enum.resource_type_enum
  }
  column "resource_id" {
    null = false
    type = uuid
  }
  column "read" {
    null    = false
    type    = boolean
    default = false
  }
  column "write" {
    null    = false
    type    = boolean
    default = false
  }
  column "execute" {
    null    = false
    type    = boolean
    default = false
  }
  primary_key {
    columns = [column.id]
  }
  foreign_key "fk_permissions_team" {
    columns     = [column.team_id]
    ref_columns = [table.team.column.id]
    on_update   = NO_ACTION
    // CASCADE: core.DeleteTeam removes the team row; stale team ACL rows would
    // otherwise fail the delete (the FK previously used NO_ACTION).
    on_delete   = CASCADE
  }
  index "idx_team_permissions_resource" {
    columns = [column.resource_type, column.resource_id]
  }
  index "uq_team_permissions_team_resource" {
    columns = [column.team_id, column.resource_type, column.resource_id]
    unique  = true
  }
}

// Table resource_sharing_rule holds the deployment-wide DEFAULT sharing rules evaluated
// when a personal or team grant does not already answer an access check (the data
// distribution layer). Seeded from useraccess.json "sharing"."defaults".
//   subject_kind = "organization_members": callers sharing an organization with the
//                  resource's owner get the rule's bits.
//   subject_kind = "team_mates": callers sharing at least one team with the owner get
//                  the rule's bits.
// resource_type NULL means the rule applies to every resource type.
table "resource_sharing_rule" {
  schema = schema.accesscontrol

  column "id" {
    null    = false
    type    = uuid
    default = sql("gen_random_uuid()")
  }
  column "resource_type" {
    null = true
    type = enum.resource_type_enum
    comment = "Restrict the rule to one resource type; NULL applies to all types."
  }
  column "subject_kind" {
    null    = false
    type    = text
    comment = "Who the rule admits relative to the owner: organization_members | team_mates."
  }
  column "read" {
    null    = false
    type    = boolean
    default = false
  }
  column "write" {
    null    = false
    type    = boolean
    default = false
  }
  column "execute" {
    null    = false
    type    = boolean
    default = false
  }
  primary_key {
    columns = [column.id]
  }
  index "uq_resource_sharing_rule" {
    columns = [column.subject_kind, column.resource_type]
    unique  = true
  }
}



schema "cron" {
  comment = "Cron schedules (file at startup + live via API, RBAC-gated via cron.*)"
}

table "schedule" {
  schema = schema.cron
  column "id" {
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "path" {
    type = text
    null = false
    comment = "Route path to invoke, e.g. /api/cleanup"
  }
  column "schedule" {
    type = text
    null = false
    comment = "Unix 5-field cron expression, e.g. 0 3 * * *"
  }
  column "expression" {
    type = text
    null = false
    comment = "Compiled EventBridge Scheduler expression, e.g. cron(0 3 * * ? *)"
  }
  column "timezone" {
    type = text
    null = false
    default = "UTC"
    comment = "Schedule timezone, e.g. UTC or Asia/Kolkata"
  }
  column "origin" {
    type = text
    null = false
    default = "seed"
    comment = "Provenance: 'seed' rows reconciled to crons.json; 'runtime' rows added via POST /crons and never touched by seeder."
  }
  column "payload" {
    type = jsonb
    null = true
    comment = "Optional JSON detail dispatched with the target. 'event:<type>' targets pass it to the event-system adapters as the event payload."
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    type = timestamp
    null = true
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_cron_schedule_path" {
    columns = [column.path]
    unique = true
  }
}



schema "event" {
  comment = "Dynamic event triggers (file at startup + live via API, RBAC-gated via event.trigger.*)"
}

table "trigger" {
  schema = schema.event
  column "id" {
    type = uuid
    default = sql("gen_random_uuid()")
  }
  column "event" {
    type = text
    null = false
  }
  column "action" {
    type = text
    null = false
  }
  column "origin" {
    type = text
    null = false
    default = "seed"
    comment = "Provenance: 'seed' rows reconciled to triggers.json by nimbus-migrate; 'runtime' rows added via POST /events/triggers and never touched by seeder."
  }
  column "created_at" {
    type = timestamp
    null = false
    default = sql("CURRENT_TIMESTAMP")
  }
  column "updated_at" {
    type = timestamp
    null = true
  }
  primary_key {
    columns = [column.id]
  }
  index "idx_event_trigger_event_action" {
    columns = [column.event, column.action]
    unique = true
  }
  index "idx_event_trigger_action" {
    columns = [column.action]
  }
}




