package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"log"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ObjectT string

var (
	kProcedure ObjectT = "PROCEDURE"
	kFunction  ObjectT = "FUNCTION"
	kTable     ObjectT = "TABLE"
)

type MySQLGrant interface {
	GetId() string
	SQLGrantStatement() string
	SQLRevokeStatement() string
	GetUserOrRole() UserOrRole
	GrantOption() bool
}

type MySQLGrantWithDatabase interface {
	MySQLGrant
	GetDatabase() string
}

type MySQLGrantWithTable interface {
	MySQLGrantWithDatabase
	GetTable() string
}

type PrivilegesPartiallyRevocable interface {
	SQLPartialRevokePrivilegesStatement(privilegesToRevoke []string) string
}

type UserOrRole interface {
	SQLString() string
}

type User struct {
	User string
	Host string
}

func (u User) SQLString() string {
	return fmt.Sprintf("'%s'@'%s'", u.User, u.Host)
}

type Role struct {
	Role string
}

func (r Role) SQLString() string {
	return fmt.Sprintf("'%s'", r.Role)
}

type TablePrivilegeGrant struct {
	Database   string
	Table      string
	Privileges []string
	Grant      bool
	UserOrRole UserOrRole
	TLSOption  string
}

func (t *TablePrivilegeGrant) GetId() string {
	if user, ok := t.UserOrRole.(User); ok {
		return fmt.Sprintf("%s@%s:%s", user.User, user.Host, t.GetDatabase())
	} else if role, ok := t.UserOrRole.(Role); ok {
		return fmt.Sprintf("%s:%s", role.Role, t.GetDatabase())
	} else {
		panic("Unknown user or role")
	}
}

func (t *TablePrivilegeGrant) GetUserOrRole() UserOrRole {
	return t.UserOrRole
}

func (t *TablePrivilegeGrant) GrantOption() bool {
	return t.Grant
}

func (t *TablePrivilegeGrant) GetDatabase() string {
	if strings.Compare(t.Database, "*") != 0 && !strings.HasSuffix(t.Database, "`") {
		return fmt.Sprintf("`%s`", t.Database)
	}
	return t.Database
}

func (t *TablePrivilegeGrant) GetTable() string {
	if t.Table == "*" || t.Table == "" {
		return "*"
	} else {
		return fmt.Sprintf("`%s`", t.Table)
	}
}

func (t *TablePrivilegeGrant) SQLGrantStatement() string {
	stmtSql := fmt.Sprintf("GRANT %s ON %s.%s TO %s", strings.Join(t.Privileges, ", "), t.GetDatabase(), t.GetTable(), t.UserOrRole.SQLString())
	if t.TLSOption != "" && strings.ToLower(t.TLSOption) != "none" {
		stmtSql += fmt.Sprintf(" REQUIRE %s", t.TLSOption)
	}
	if t.Grant {
		stmtSql += " WITH GRANT OPTION"
	}
	return stmtSql
}

func (t *TablePrivilegeGrant) SQLRevokeStatement() string {
	stmt := fmt.Sprintf("REVOKE %s ON %s.%s FROM %s", strings.Join(t.Privileges, ", "), t.GetDatabase(), t.GetTable(), t.UserOrRole.SQLString())
	if t.Grant {
		stmt += " WITH GRANT OPTION"
	}
	return stmt
}

func (t *TablePrivilegeGrant) SQLPartialRevokePrivilegesStatement(privilegesToRevoke []string) string {
	stmt := fmt.Sprintf("REVOKE %s ON %s.%s FROM %s", strings.Join(privilegesToRevoke, ", "), t.GetDatabase(), t.GetTable(), t.UserOrRole.SQLString())
	if t.Grant {
		stmt += " WITH GRANT OPTION"
	}
	return stmt
}

type ProcedurePrivilegeGrant struct {
	Database     string
	ObjectT      ObjectT
	CallableName string
	Privileges   []string
	Grant        bool
	UserOrRole   UserOrRole
	TLSOption    string
}

func (t *ProcedurePrivilegeGrant) GetId() string {
	if user, ok := t.UserOrRole.(User); ok {
		return fmt.Sprintf("%s@%s:%s", user.User, user.Host, t.GetDatabase())
	} else if role, ok := t.UserOrRole.(Role); ok {
		return fmt.Sprintf("%s:%s", role.Role, t.GetDatabase())
	} else {
		panic("Unknown user or role")
	}
}

func (t *ProcedurePrivilegeGrant) GetUserOrRole() UserOrRole {
	return t.UserOrRole
}

func (t *ProcedurePrivilegeGrant) GrantOption() bool {
	return t.Grant
}

func (t *ProcedurePrivilegeGrant) GetDatabase() string {
	if strings.Compare(t.Database, "*") != 0 && !strings.HasSuffix(t.Database, "`") {
		return fmt.Sprintf("`%s`", t.Database)
	}
	return t.Database
}

func (t *ProcedurePrivilegeGrant) SQLGrantStatement() string {
	stmtSql := fmt.Sprintf("GRANT %s ON %s %s.%s TO %s", strings.Join(t.Privileges, ", "), t.ObjectT, t.GetDatabase(), t.CallableName, t.UserOrRole.SQLString())
	if t.TLSOption != "" && strings.ToLower(t.TLSOption) != "none" {
		stmtSql += fmt.Sprintf(" REQUIRE %s", t.TLSOption)
	}
	if t.Grant {
		stmtSql += " WITH GRANT OPTION"
	}
	return stmtSql
}

func (t *ProcedurePrivilegeGrant) SQLRevokeStatement() string {
	stmt := fmt.Sprintf("REVOKE %s ON %s %s.%s FROM %s", strings.Join(t.Privileges, ", "), t.ObjectT, t.GetDatabase(), t.CallableName, t.UserOrRole.SQLString())
	if t.Grant {
		stmt += " WITH GRANT OPTION"
	}
	return stmt
}

func (t *ProcedurePrivilegeGrant) SQLPartialRevokePrivilegesStatement(privilegesToRevoke []string) string {
	stmt := fmt.Sprintf("REVOKE %s ON %s %s.%s FROM %s", strings.Join(privilegesToRevoke, ", "), t.ObjectT, t.GetDatabase(), t.CallableName, t.UserOrRole.SQLString())
	if t.Grant {
		stmt += " WITH GRANT OPTION"
	}
	return stmt
}

type RoleGrant struct {
	Roles      []string
	Grant      bool
	UserOrRole UserOrRole
	TLSOption  string
}

func (t *RoleGrant) GetId() string {
	if user, ok := t.UserOrRole.(User); ok {
		return fmt.Sprintf("%s@%s", user.User, user.Host)
	} else if role, ok := t.UserOrRole.(Role); ok {
		return fmt.Sprintf("%s", role.Role)
	} else {
		panic("Unknown user or role")
	}
}

func (t *RoleGrant) GetUserOrRole() UserOrRole {
	return t.UserOrRole
}

func (t *RoleGrant) GrantOption() bool {
	return t.Grant
}

func (t *RoleGrant) SQLGrantStatement() string {
	stmtSql := fmt.Sprintf("GRANT %s TO %s", strings.Join(t.Roles, ", "), t.UserOrRole.SQLString())
	if t.TLSOption != "" && strings.ToLower(t.TLSOption) != "none" {
		stmtSql += fmt.Sprintf(" REQUIRE %s", t.TLSOption)
	}
	if t.Grant {
		stmtSql += " WITH ADMIN OPTION"
	}
	return stmtSql
}

func (t *RoleGrant) SQLRevokeStatement() string {
	return fmt.Sprintf("REVOKE %s FROM %s", strings.Join(t.Roles, ", "), t.UserOrRole.SQLString())
}

func resourceGrant() *schema.Resource {
	return &schema.Resource{
		CreateContext: CreateGrant,
		UpdateContext: UpdateGrant,
		ReadContext:   ReadGrant,
		DeleteContext: DeleteGrant,
		Importer: &schema.ResourceImporter{
			StateContext: ImportGrant,
		},

		Schema: map[string]*schema.Schema{
			"user": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"role"},
			},

			"role": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"user", "host"},
			},

			"host": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Default:       "localhost",
				ConflictsWith: []string{"role"},
			},

			"database": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"table": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "*",
			},

			"privileges": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"roles": {
				Type:          schema.TypeSet,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"privileges"},
				Elem:          &schema.Schema{Type: schema.TypeString},
				Set:           schema.HashString,
			},

			"grant": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
				Default:  false,
			},

			"tls_option": {
				Type:       schema.TypeString,
				Optional:   true,
				ForceNew:   true,
				Deprecated: "Please use tls_option in mysql_user.",
				Default:    "NONE",
			},
		},
	}
}

func supportsRoles(ctx context.Context, meta interface{}) (bool, error) {
	currentVersion := getVersionFromMeta(ctx, meta)

	requiredVersion, _ := version.NewVersion("8.0.0")
	hasRoles := currentVersion.GreaterThan(requiredVersion)
	return hasRoles, nil
}

var kReProcedureWithoutDatabase = regexp.MustCompile(`(?i)^(function|procedure) ([^.]*)$`)
var kReProcedureWithDatabase = regexp.MustCompile(`(?i)^(function|procedure) ([^.]*)\.([^.]*)$`)

func parseResourceFromData(d *schema.ResourceData) (MySQLGrant, diag.Diagnostics) {

	// Step 1: Parse the user/role
	var userOrRole UserOrRole
	userAttr, userOk := d.GetOk("user")
	hostAttr, hostOk := d.GetOk("host")
	roleAttr, roleOk := d.GetOk("role")
	if userOk && hostOk && userAttr.(string) != "" && hostAttr.(string) != "" {
		userOrRole = User{
			User: userAttr.(string),
			Host: hostAttr.(string),
		}
	} else if roleOk && roleAttr.(string) != "" {
		userOrRole = Role{
			Role: roleAttr.(string),
		}
	} else {
		return nil, diag.Errorf("One of user/host or role is required")
	}

	// Step 2: Get generic attributes
	database := d.Get("database").(string)
	tlsOption := d.Get("tls_option").(string)
	grantOption := d.Get("grant").(bool)

	// Step 3a: If `roles` is specified, we have a role grant
	if attr, ok := d.GetOk("roles"); ok {
		roles := setToArray(attr)
		return &RoleGrant{
			Roles:      roles,
			Grant:      grantOption,
			UserOrRole: userOrRole,
			TLSOption:  tlsOption,
		}, nil
	}

	// Step 3b. If the database is a procedure or function, we have a procedure grant
	if kReProcedureWithDatabase.MatchString(database) || kReProcedureWithoutDatabase.MatchString(database) {
		var callableType ObjectT
		var callableName string
		if kReProcedureWithDatabase.MatchString(database) {
			matches := kReProcedureWithDatabase.FindStringSubmatch(database)
			callableType = ObjectT(matches[1])
			database = matches[2]
			callableName = matches[3]
		} else {
			matches := kReProcedureWithoutDatabase.FindStringSubmatch(database)
			callableType = ObjectT(matches[1])
			database = matches[2]
			callableName = d.Get("table").(string)
		}
		privsList := setToArray(d.Get("privileges"))
		privileges := normalizePerms(privsList)

		return &ProcedurePrivilegeGrant{
			Database:     database,
			ObjectT:      callableType,
			CallableName: callableName,
			Privileges:   privileges,
			Grant:        grantOption,
			UserOrRole:   userOrRole,
			TLSOption:    tlsOption,
		}, nil
	}

	// Step 3c. Otherwise, we have a table grant
	privsList := setToArray(d.Get("privileges"))
	privileges := normalizePerms(privsList)

	return &TablePrivilegeGrant{
		Database:   database,
		Table:      d.Get("table").(string),
		Privileges: privileges,
		Grant:      grantOption,
		UserOrRole: userOrRole,
		TLSOption:  tlsOption,
	}, nil
}

func CreateGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	// Parse the ResourceData
	grant, diagErr := parseResourceFromData(d)
	if err != nil {
		return diagErr
	}

	// Determine whether the database has support for roles
	hasRolesSupport, err := supportsRoles(ctx, meta)
	if err != nil {
		return diag.Errorf("failed getting role support: %v", err)
	}
	if _, ok := grant.(*RoleGrant); ok && !hasRolesSupport {
		return diag.Errorf("role grants are not supported by this version of MySQL")
	}

	// Check to see if there are existing roles that might be clobbered by this grant
	hasConflicts, err := hasConflictingGrants(ctx, db, grant)
	if err != nil {
		return diag.Errorf("failed showing grants: %v", err)
	}
	if hasConflicts {
		return diag.Errorf("user/role %s already has unmanaged grant - import it first", grant.GetUserOrRole())
	}

	stmtSQL := grant.SQLGrantStatement()

	log.Println("Executing statement:", stmtSQL)
	_, err = db.ExecContext(ctx, stmtSQL)
	if err != nil {
		return diag.Errorf("Error running SQL (%s): %s", stmtSQL, err)
	}

	d.SetId(grant.GetId())
	return ReadGrant(ctx, d, meta)
}

func ReadGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.Errorf("failed getting database from Meta: %v", err)
	}

	grant, diagErr := parseResourceFromData(d)
	if diagErr != nil {
		return diagErr
	}

	allGrants, err := showUserGrants(ctx, db, grant.GetUserOrRole())
	if err != nil {
		return diag.Errorf("showGrant - getting all grants failed: %w", err)
	}

	if len(allGrants) == 0 {
		log.Printf("[WARN] GRANT not found for %s - removing from state", grant.GetUserOrRole())
		d.SetId("")
		return nil
	}

	setDataFromGrant(grant, d)

	return nil
}

func UpdateGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	if err != nil {
		return diag.Errorf("failed getting user or role: %v", err)
	}

	if d.HasChange("privileges") {
		grant, diagErr := parseResourceFromData(d)
		if diagErr != nil {
			return diagErr
		}

		err = updatePrivileges(ctx, db, d, grant)
		if err != nil {
			return diag.Errorf("failed updating privileges: %v", err)
		}
	}

	return nil
}

func updatePrivileges(ctx context.Context, db *sql.DB, d *schema.ResourceData, grant MySQLGrant) error {
	oldPrivsIf, newPrivsIf := d.GetChange("privileges")
	oldPrivs := oldPrivsIf.(*schema.Set)
	newPrivs := newPrivsIf.(*schema.Set)
	grantIfs := newPrivs.Difference(oldPrivs).List()
	revokeIfs := oldPrivs.Difference(newPrivs).List()

	// Do a partial revoke of anything that has been removed
	if len(revokeIfs) > 0 {
		revokes := make([]string, len(revokeIfs))
		for i, v := range revokeIfs {
			revokes[i] = v.(string)
		}

		partialRevoker, ok := grant.(PrivilegesPartiallyRevocable)
		if !ok {
			return fmt.Errorf("grant does not support partial privilege revokes")
		}
		sqlCommand := partialRevoker.SQLPartialRevokePrivilegesStatement(revokes)
		log.Printf("[DEBUG] SQL: %s", sqlCommand)

		if _, err := db.ExecContext(ctx, sqlCommand); err != nil {
			return err
		}
	}

	// Do a full grant if anything has been added
	if len(grantIfs) > 0 {
		sqlCommand := grant.SQLGrantStatement()
		log.Printf("[DEBUG] SQL: %s", sqlCommand)

		if _, err := db.ExecContext(ctx, sqlCommand); err != nil {
			return err
		}
	}

	return nil
}

func DeleteGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return diag.FromErr(err)
	}

	grant, diagErr := parseResourceFromData(d)
	if err != nil {
		return diagErr
	}

	sqlStatement := grant.SQLRevokeStatement()
	log.Printf("[DEBUG] SQL: %s", sqlStatement)
	_, err = db.ExecContext(ctx, sqlStatement)
	if err != nil {
		if !isNonExistingGrant(err) {
			return diag.Errorf("error revoking ALL (%s): %s", sqlStatement, err)
		}
	}

	return nil
}

func isNonExistingGrant(err error) bool {
	if driverErr, ok := err.(*mysql.MySQLError); ok {
		// 1141 = ER_NONEXISTING_GRANT
		// 1147 = ER_NONEXISTING_TABLE_GRANT
		// 1403 = ER_NONEXISTING_PROC_GRANT

		if driverErr.Number == 1141 || driverErr.Number == 1147 || driverErr.Number == 1403 {
			return true
		}
	}
	return false
}

func ImportGrant(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	userHostDatabaseTable := strings.Split(d.Id(), "@")

	if len(userHostDatabaseTable) != 4 && len(userHostDatabaseTable) != 5 {
		return nil, fmt.Errorf("wrong ID format %s - expected user@host@database@table (and optionally ending @ to signify grant option) where some parts can be empty)", d.Id())
	}

	user := userHostDatabaseTable[0]
	host := userHostDatabaseTable[1]
	database := userHostDatabaseTable[2]
	table := userHostDatabaseTable[3]
	grantOption := len(userHostDatabaseTable) == 5

	userOrRole := User{
		User: user,
		Host: host,
	}

	db, err := getDatabaseFromMeta(ctx, meta)
	if err != nil {
		return nil, err
	}

	grants, err := showUserGrants(ctx, db, userOrRole)
	if err != nil {
		return nil, err
	}
	for _, grant := range grants {
		// Grant options must match
		if grant.GrantOption() != grantOption {
			continue
		}

		// If we have a database grant, we need to match the database name
		if dbGrant, ok := grant.(MySQLGrantWithDatabase); (!ok && database != "") || (ok && dbGrant.GetDatabase() != database) {
			continue
		}

		// If we have a table grant, we need to match the table name
		if tableGrant, ok := grant.(MySQLGrantWithTable); (!ok && table != "") || (ok && tableGrant.GetTable() != table) {
			continue
		}

		// We have a match!
		res := resourceGrant().Data(nil)
		setDataFromGrant(grant, res)
		return []*schema.ResourceData{res}, nil
	}

	// No match found
	results := []*schema.ResourceData{}
	return results, nil
}

// setDataFromGrant copies the values from MySQLGrant to the schema.ResourceData
func setDataFromGrant(grant MySQLGrant, d *schema.ResourceData) *schema.ResourceData {
	if tableGrant, ok := grant.(*TablePrivilegeGrant); ok {
		d.Set("database", tableGrant.Database)
		d.Set("table", tableGrant.Table)
		d.Set("grant", grant.GrantOption())
		d.Set("privileges", tableGrant.Privileges)
		d.Set("tls_option", tableGrant.TLSOption)
	} else if procedureGrant, ok := grant.(*ProcedurePrivilegeGrant); ok {
		d.Set("database", fmt.Sprintf("%s %s.%s", procedureGrant.ObjectT, procedureGrant.Database, procedureGrant.CallableName))
		d.Set("table", "")
		d.Set("grant", grant.GrantOption())
		d.Set("privileges", procedureGrant.Privileges)
		d.Set("tls_option", procedureGrant.TLSOption)
	} else if roleGrant, ok := grant.(*RoleGrant); ok {
		d.Set("grant", grant.GrantOption())
		d.Set("roles", roleGrant.Roles)
		d.Set("tls_option", roleGrant.TLSOption)
	} else {
		panic("Unknown grant type")
	}

	if user, ok := grant.GetUserOrRole().(User); ok {
		d.Set("user", user.User)
		d.Set("host", user.Host)
	} else if role, ok := grant.GetUserOrRole().(Role); ok {
		d.Set("role", role.Role)
	} else {
		panic("Unknown user or role")
	}

	return d
}

func hasConflictingGrants(ctx context.Context, db *sql.DB, desiredGrant MySQLGrant) (bool, error) {
	allGrants, err := showUserGrants(ctx, db, desiredGrant.GetUserOrRole())
	if err != nil {
		return false, fmt.Errorf("showGrant - getting all grants failed: %w", err)
	}
	for _, dbGrant := range allGrants {
		if desiredGrant.GrantOption() != dbGrant.GrantOption() {
			continue
		}
		if reflect.TypeOf(desiredGrant) == reflect.TypeOf(dbGrant) {
			return true, nil
		}
	}
	return false, nil
}

var (
	reRequire = regexp.MustCompile(`.*REQUIRE\s+(.*)`)

	userHostRegex = regexp.MustCompile(`'([^']*)'(@'([^']*)')?`)
	roleRegex     = regexp.MustCompile(`'[^']+'`)

	roleGrantRegex      = regexp.MustCompile(`GRANT\s+([\w\s,]+)\s+TO\s+(.+)`)
	reGrant             = regexp.MustCompile(`\bGRANT OPTION\b|\bADMIN OPTION\b`)
	procedureGrantRegex = regexp.MustCompile(`GRANT\s+([\w\s,]+)\s+ON\s+(FUNCTION|PROCEDURE)\s+([\w\s,]+)\s+TO\s+(.+)`)
	tableGrantRegex     = regexp.MustCompile(`GRANT\s+([\w\s,]+)\s+ON\s+(.+)\s+TO\s+(.+)`)
)

func parseGrantFromRow(grantStr string) (MySQLGrant, error) {

	// Ignore REVOKE.*
	if strings.HasPrefix(grantStr, "REVOKE") {
		log.Printf("[WARN] Partial revokes are not fully supported and lead to unexpected behavior. Consult documentation https://dev.mysql.com/doc/refman/8.0/en/partial-revokes.html on how to disable them for safe and reliable terraform. Relevant partial revoke: %s\n", grantStr)
		return nil, nil
	}

	// Parse Require Statement
	tlsOption := ""
	if requireMatches := reRequire.FindStringSubmatch(grantStr); len(requireMatches) == 2 {
		tlsOption = requireMatches[1]
	}

	// Parse User or Role Statement
	var userOrRole UserOrRole
	if userHostMatches := userHostRegex.FindStringSubmatch(grantStr); len(userHostMatches) == 4 {
		user := strings.Trim(userHostMatches[1], "`' ")
		host := strings.Trim(userHostMatches[3], "`' ")
		userOrRole = User{
			User: user,
			Host: host,
		}
	} else if roleMatches := roleRegex.FindStringSubmatch(grantStr); len(roleMatches) == 1 {
		role := strings.Trim(roleMatches[0], "`' ")
		userOrRole = Role{
			Role: role,
		}
	} else {
		return nil, fmt.Errorf("failed to parse grant statement: %s", grantStr)
	}

	// Handle Role Grants
	if roleMatches := roleGrantRegex.FindStringSubmatch(grantStr); len(roleMatches) == 3 {
		rolesStart := strings.Split(roleMatches[1], ",")
		roles := make([]string, len(rolesStart))

		for i, role := range rolesStart {
			roles[i] = strings.Trim(role, "`@%\" ")
		}

		grant := &RoleGrant{
			Roles:      roles,
			Grant:      reGrant.MatchString(grantStr),
			UserOrRole: userOrRole,
			TLSOption:  tlsOption,
		}
		return grant, nil

	} else if procedureMatches := procedureGrantRegex.FindStringSubmatch(grantStr); len(procedureMatches) == 5 {
		privsStr := procedureMatches[1]
		privileges := extractPermTypes(privsStr)
		privileges = normalizePerms(privileges)

		grant := &ProcedurePrivilegeGrant{
			Database:     strings.Trim(procedureMatches[3], "`\""),
			ObjectT:      ObjectT(procedureMatches[2]),
			CallableName: strings.Trim(procedureMatches[3], "`\""),
			Privileges:   privileges,
			Grant:        reGrant.MatchString(grantStr),
			UserOrRole:   userOrRole,
			TLSOption:    tlsOption,
		}
		return grant, nil
	} else if tableMatches := tableGrantRegex.FindStringSubmatch(grantStr); len(tableMatches) == 4 {
		privsStr := tableMatches[1]
		privileges := extractPermTypes(privsStr)
		privileges = normalizePerms(privileges)

		grant := &TablePrivilegeGrant{
			Database:   strings.Trim(tableMatches[2], "`\""),
			Table:      strings.Trim(tableMatches[3], "`\""),
			Privileges: privileges,
			Grant:      reGrant.MatchString(grantStr),
			UserOrRole: userOrRole,
			TLSOption:  tlsOption,
		}
		return grant, nil
	} else {
		return nil, fmt.Errorf("failed to parse grant statement: %s", grantStr)
	}
}

func showUserGrants(ctx context.Context, db *sql.DB, userOrRole UserOrRole) ([]MySQLGrant, error) {
	grants := []MySQLGrant{}

	sqlStatement := fmt.Sprintf("SHOW GRANTS FOR %s", userOrRole.SQLString())
	log.Printf("[DEBUG] SQL: %s", sqlStatement)
	rows, err := db.QueryContext(ctx, sqlStatement)

	if isNonExistingGrant(err) {
		return []MySQLGrant{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("showUserGrants - getting grants failed: %w", err)
	}

	defer rows.Close()
	for rows.Next() {
		var rawGrant string

		err := rows.Scan(&rawGrant)
		if err != nil {
			return nil, fmt.Errorf("showUserGrants - reading row failed: %w", err)
		}

		parsedGrant, err := parseGrantFromRow(rawGrant)
		if err != nil {
			return nil, err
		}
		if parsedGrant == nil {
			continue
		}

		// Filter out any grants that don't match the provided user
		// Percona returns also grants for % if we requested IP.
		// Skip them as we don't want terraform to consider it.
		if parsedGrant.GetUserOrRole().SQLString() != userOrRole.SQLString() {
			log.Printf("[DEBUG] Skipping grant for %s as it doesn't match %s", parsedGrant.GetUserOrRole().SQLString(), userOrRole.SQLString())
			continue
		}
		grants = append(grants, parsedGrant)

	}
	log.Printf("[DEBUG] Parsed grants are: %v", grants)
	return grants, nil
}

func removeUselessPerms(grants []string) []string {
	ret := []string{}
	for _, grant := range grants {
		if grant != "USAGE" {
			ret = append(ret, grant)
		}
	}
	return ret
}

func extractPermTypes(g string) []string {
	grants := []string{}

	inParentheses := false
	currentWord := []rune{}
	for _, b := range g {
		switch b {
		case ',':
			if inParentheses {
				currentWord = append(currentWord, b)
			} else {
				grants = append(grants, string(currentWord))
				currentWord = []rune{}
			}
			break
		case '(':
			inParentheses = true
			currentWord = append(currentWord, b)
			break
		case ')':
			inParentheses = false
			currentWord = append(currentWord, b)
			break
		default:
			if unicode.IsSpace(b) && len(currentWord) == 0 {
				break
			}
			currentWord = append(currentWord, b)
		}
	}
	grants = append(grants, string(currentWord))
	return grants
}

func normalizeColumnOrder(perm string) string {
	re := regexp.MustCompile("^([^(]*)\\((.*)\\)$")
	// We may get inputs like
	// 	SELECT(b,a,c)   -> SELECT(a,b,c)
	// 	DELETE          -> DELETE
	// if it's without parentheses, return it right away.
	// Else split what is inside, sort it, concat together and return the result.
	m := re.FindStringSubmatch(perm)
	if m == nil || len(m) < 3 {
		return perm
	}

	parts := strings.Split(m[2], ",")
	for i := range parts {
		parts[i] = strings.Trim(parts[i], "` ")
	}
	sort.Strings(parts)
	partsTogether := strings.Join(parts, ", ")
	return fmt.Sprintf("%s(%s)", m[1], partsTogether)
}

func normalizePerms(perms []string) []string {
	// Spaces and backticks are optional, let's ignore them.
	re := regexp.MustCompile("[ `]")
	ret := []string{}
	for _, perm := range perms {
		permNorm := re.ReplaceAllString(perm, "")
		permUcase := strings.ToUpper(permNorm)
		if permUcase == "ALL" || permUcase == "ALLPRIVILEGES" {
			permUcase = "ALL PRIVILEGES"
		}
		permSortedColumns := normalizeColumnOrder(permUcase)

		ret = append(ret, permSortedColumns)
	}
	ret = removeUselessPerms(ret)
	return ret
}

func setToArray(s interface{}) []string {
	set, ok := s.(*schema.Set)
	if !ok {
		return []string{}
	}

	ret := []string{}
	for _, elem := range set.List() {
		ret = append(ret, elem.(string))
	}
	return ret
}
