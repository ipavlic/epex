package interpreter

import (
	"testing"

	"github.com/ipavlic/epex/apex"
	"github.com/ipavlic/epex/engine"
	"github.com/ipavlic/epex/schema"
)

// setupWithRealEngineMulti creates an interpreter with a real SQLite engine and
// registers multiple Apex classes parsed from separate source strings.
func setupWithRealEngineMulti(t *testing.T, sources ...string) *Interpreter {
	t.Helper()
	s, err := schema.BuildSchemaFromDir("../testdata/objects")
	if err != nil {
		t.Fatalf("building schema: %v", err)
	}
	eng, err := engine.NewEngine(s)
	if err != nil {
		t.Fatalf("creating engine: %v", err)
	}
	t.Cleanup(func() { eng.Close() })
	reg := NewRegistry()
	for _, src := range sources {
		result, err := apex.ParseString("test.cls", src)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		reg.RegisterClass(result.Tree)
	}
	return NewInterpreter(reg, eng)
}

// --- Phase 1: runAs ---

func TestRunAsChangesUserInfo(t *testing.T) {
	source := `
@isTest
public class RunAsTest {
    public static String testRunAs() {
        User u = new User();
        u.Username = 'alice@test.com';
        u.ProfileId = '00e000000000099';
        insert u;

        String uid;
        System.runAs(u) {
            uid = UserInfo.getUserId();
        }
        return uid;
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	result := interp.ExecuteMethod("RunAsTest", "testRunAs", nil)
	if result.Type != TypeString {
		t.Fatalf("expected string, got %v", result.Type)
	}
	val := result.Data.(string)
	if val == "" || val == "005000000000001" {
		t.Errorf("expected runAs user Id, got %q", val)
	}
}

func TestRunAsRestoresContext(t *testing.T) {
	source := `
@isTest
public class RunAsTest {
    public static String testRestore() {
        User u = new User();
        u.Username = 'bob@test.com';
        insert u;

        System.runAs(u) {
            // inside runAs
        }
        return UserInfo.getUserId();
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	result := interp.ExecuteMethod("RunAsTest", "testRestore", nil)
	if result.Data.(string) != "005000000000001" {
		t.Errorf("expected default user Id after runAs, got %q", result.Data.(string))
	}
}

func TestRunAsNested(t *testing.T) {
	source := `
@isTest
public class RunAsTest {
    public static String testNested() {
        User u1 = new User();
        u1.Username = 'user1@test.com';
        insert u1;

        User u2 = new User();
        u2.Username = 'user2@test.com';
        insert u2;

        String innerUid;
        System.runAs(u1) {
            System.runAs(u2) {
                innerUid = UserInfo.getUserId();
            }
        }
        return innerUid;
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	result := interp.ExecuteMethod("RunAsTest", "testNested", nil)
	val := result.Data.(string)
	// Should be u2's Id, not u1's and not default
	if val == "" || val == "005000000000001" {
		t.Errorf("expected u2's Id in nested runAs, got %q", val)
	}
}

func TestRunAsUserInfoUsername(t *testing.T) {
	source := `
@isTest
public class RunAsTest {
    public static String testUsername() {
        User u = new User();
        u.Username = 'specific@test.com';
        insert u;

        String username;
        System.runAs(u) {
            username = UserInfo.getUserName();
        }
        return username;
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	result := interp.ExecuteMethod("RunAsTest", "testUsername", nil)
	if result.Data.(string) != "specific@test.com" {
		t.Errorf("expected 'specific@test.com', got %q", result.Data.(string))
	}
}

func TestRunAsUserInfoProfileId(t *testing.T) {
	source := `
@isTest
public class RunAsTest {
    public static String testProfileId() {
        User u = new User();
        u.Username = 'proftest@test.com';
        u.ProfileId = '00e000000000077';
        insert u;

        String pid;
        System.runAs(u) {
            pid = UserInfo.getProfileId();
        }
        return pid;
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	result := interp.ExecuteMethod("RunAsTest", "testProfileId", nil)
	if result.Data.(string) != "00e000000000077" {
		t.Errorf("expected '00e000000000077', got %q", result.Data.(string))
	}
}

// --- Phase 2: Sharing ---

func TestWithSharingFiltersRecords(t *testing.T) {
	classSource := `
public with sharing class SharingTest {
    public static Integer countAccounts() {
        List<Account> accs = [SELECT Id FROM Account];
        return accs.size();
    }
}`
	interp := setupWithRealEngineMulti(t, classSource)

	// Insert 2 accounts with different owners
	interp.engine.Insert("Account", []map[string]any{
		{"Name": "Acc1", "OwnerId": "005OWNER1"},
		{"Name": "Acc2", "OwnerId": "005OWNER2"},
	})

	// Without runAs, should see all records (no user context)
	result := interp.ExecuteMethod("SharingTest", "countAccounts", nil)
	if result.Data.(int) != 2 {
		t.Errorf("without runAs, expected 2 records, got %v", result.Data)
	}

	// With runAs as OWNER1, "with sharing" should filter to owned records only
	interp.execCtx = &executionContext{
		userID:     "005OWNER1",
		userFields: map[string]*Value{},
	}
	result = interp.ExecuteMethod("SharingTest", "countAccounts", nil)
	interp.execCtx = nil
	if result.Data.(int) != 1 {
		t.Errorf("with sharing + runAs, expected 1 record, got %v", result.Data)
	}
}

func TestWithoutSharingSeesAll(t *testing.T) {
	classSource := `
public without sharing class NoSharingTest {
    public static Integer countAccounts() {
        List<Account> accs = [SELECT Id FROM Account];
        return accs.size();
    }
}`
	interp := setupWithRealEngineMulti(t, classSource)

	interp.engine.Insert("Account", []map[string]any{
		{"Name": "Acc1", "OwnerId": "005OWNER1"},
		{"Name": "Acc2", "OwnerId": "005OWNER2"},
	})

	// With runAs but "without sharing", should see all records
	interp.execCtx = &executionContext{
		userID:     "005OWNER1",
		userFields: map[string]*Value{},
	}
	result := interp.ExecuteMethod("NoSharingTest", "countAccounts", nil)
	interp.execCtx = nil
	if result.Data.(int) != 2 {
		t.Errorf("without sharing + runAs, expected 2 records, got %v", result.Data)
	}
}

func TestInsertSetsOwnerId(t *testing.T) {
	source := `
@isTest
public class OwnerTest {
    public static String testOwnerId() {
        User u = new User();
        u.Username = 'owner@test.com';
        insert u;

        Account acc = new Account();
        acc.Name = 'Owned Account';
        System.runAs(u) {
            insert acc;
        }
        return (String)acc.get('OwnerId');
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	result := interp.ExecuteMethod("OwnerTest", "testOwnerId", nil)
	val := result.Data.(string)
	if val == "" {
		t.Error("expected OwnerId to be set, got empty string")
	}
}

// --- Phase 3: Permissions ---

func TestDMLAsUserChecksCRUD(t *testing.T) {
	source := `
@isTest
public class CrudTest {
    public static void testInsertAsUser() {
        Profile p = new Profile();
        p.Name = 'Restricted';
        insert p;

        // Create ObjectPermissions denying create on Account
        ObjectPermissions op = new ObjectPermissions();
        op.ParentId = p.Id;
        op.SobjectType = 'Account';
        op.PermissionsCreate = false;
        op.PermissionsRead = true;
        op.PermissionsEdit = false;
        op.PermissionsDelete = false;
        insert op;

        User u = new User();
        u.Username = 'restricted@test.com';
        u.ProfileId = p.Id;
        insert u;

        System.runAs(u) {
            Account acc = new Account();
            acc.Name = 'Should Fail';
            insert as user acc;
        }
    }
}`
	interp := setupWithRealEngineMulti(t, source)

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected INSERT AS USER to throw on restricted profile")
			return
		}
		ts, ok := r.(*ThrowSignal)
		if !ok {
			t.Errorf("expected ThrowSignal, got %T: %v", r, r)
			return
		}
		if ts.Value.Type != TypeString {
			t.Errorf("expected string error, got %v", ts.Value)
		}
	}()
	interp.ExecuteMethod("CrudTest", "testInsertAsUser", nil)
}

func TestDMLAsUserAllowsWhenPermitted(t *testing.T) {
	source := `
@isTest
public class CrudTest {
    public static void testInsertAllowed() {
        Profile p = new Profile();
        p.Name = 'Allowed';
        insert p;

        ObjectPermissions op = new ObjectPermissions();
        op.ParentId = p.Id;
        op.SobjectType = 'Account';
        op.PermissionsCreate = true;
        op.PermissionsRead = true;
        op.PermissionsEdit = true;
        op.PermissionsDelete = true;
        insert op;

        User u = new User();
        u.Username = 'allowed@test.com';
        u.ProfileId = p.Id;
        insert u;

        System.runAs(u) {
            Account acc = new Account();
            acc.Name = 'Should Work';
            insert as user acc;
        }
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	// Should not panic
	interp.ExecuteMethod("CrudTest", "testInsertAllowed", nil)
}

func TestDMLAsSystemBypassesChecks(t *testing.T) {
	source := `
@isTest
public class CrudTest {
    public static void testInsertAsSystem() {
        Profile p = new Profile();
        p.Name = 'Restricted';
        insert p;

        ObjectPermissions op = new ObjectPermissions();
        op.ParentId = p.Id;
        op.SobjectType = 'Account';
        op.PermissionsCreate = false;
        op.PermissionsRead = true;
        op.PermissionsEdit = false;
        op.PermissionsDelete = false;
        insert op;

        User u = new User();
        u.Username = 'sysuser@test.com';
        u.ProfileId = p.Id;
        insert u;

        System.runAs(u) {
            Account acc = new Account();
            acc.Name = 'System Insert';
            insert as system acc;
        }
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	// INSERT AS SYSTEM should not enforce CRUD checks, so no panic
	interp.ExecuteMethod("CrudTest", "testInsertAsSystem", nil)
}

func TestSOQLWithSecurityEnforced(t *testing.T) {
	source := `
@isTest
public class FlsTest {
    public static void testSecurityEnforced() {
        Profile p = new Profile();
        p.Name = 'FlsRestricted';
        insert p;

        FieldPermissions fp = new FieldPermissions();
        fp.ParentId = p.Id;
        fp.SobjectType = 'Account';
        fp.Field = 'Account.Industry';
        fp.PermissionsRead = false;
        fp.PermissionsEdit = false;
        insert fp;

        User u = new User();
        u.Username = 'flsuser@test.com';
        u.ProfileId = p.Id;
        insert u;

        Account acc = new Account();
        acc.Name = 'FLS Test';
        insert acc;

        System.runAs(u) {
            List<Account> accs = [SELECT Id, Industry FROM Account WITH SECURITY_ENFORCED];
        }
    }
}`
	interp := setupWithRealEngineMulti(t, source)

	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected SECURITY_ENFORCED to throw on inaccessible field")
			return
		}
		ts, ok := r.(*ThrowSignal)
		if !ok {
			t.Errorf("expected ThrowSignal, got %T: %v", r, r)
			return
		}
		msg := ts.Value.ToString()
		if msg == "" {
			t.Error("expected error message about field privileges")
		}
	}()
	interp.ExecuteMethod("FlsTest", "testSecurityEnforced", nil)
}

func TestSOQLWithUserModeThrowsOnInaccessibleField(t *testing.T) {
	source := `
@isTest
public class FlsTest {
    public static void testUserMode() {
        Profile p = new Profile();
        p.Name = 'FlsRestricted2';
        insert p;

        FieldPermissions fp = new FieldPermissions();
        fp.ParentId = p.Id;
        fp.SobjectType = 'Account';
        fp.Field = 'Account.Industry';
        fp.PermissionsRead = false;
        fp.PermissionsEdit = false;
        insert fp;

        User u = new User();
        u.Username = 'flsuser2@test.com';
        u.ProfileId = p.Id;
        insert u;

        Account acc = new Account();
        acc.Name = 'UserMode Test';
        acc.Industry = 'Tech';
        insert acc;

        System.runAs(u) {
            List<Account> accs = [SELECT Id, Name, Industry FROM Account WITH USER_MODE];
        }
    }
}`
	interp := setupWithRealEngineMulti(t, source)

	// USER_MODE throws on inaccessible fields (same as SECURITY_ENFORCED).
	// Stripping fields silently is the behavior of stripInaccessible(), not USER_MODE.
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected USER_MODE to throw on inaccessible field")
			return
		}
		ts, ok := r.(*ThrowSignal)
		if !ok {
			t.Errorf("expected ThrowSignal, got %T: %v", r, r)
			return
		}
		msg := ts.Value.ToString()
		if msg == "" {
			t.Error("expected error message about field privileges")
		}
	}()
	interp.ExecuteMethod("FlsTest", "testUserMode", nil)
}

func TestDescribePermissionAware(t *testing.T) {
	source := `
@isTest
public class DescribeTest {
    public static Boolean testIsCreateable() {
        Profile p = new Profile();
        p.Name = 'NoCreate';
        insert p;

        ObjectPermissions op = new ObjectPermissions();
        op.ParentId = p.Id;
        op.SobjectType = 'Account';
        op.PermissionsCreate = false;
        op.PermissionsRead = true;
        op.PermissionsEdit = true;
        op.PermissionsDelete = true;
        insert op;

        User u = new User();
        u.Username = 'nocreate@test.com';
        u.ProfileId = p.Id;
        insert u;

        Boolean result;
        System.runAs(u) {
            Map<String, Schema.SObjectType> gd = Schema.getGlobalDescribe();
            Schema.SObjectType accType = gd.get('account');
            Schema.DescribeSObjectResult descr = accType.getDescribe();
            result = descr.isCreateable();
        }
        return result;
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	result := interp.ExecuteMethod("DescribeTest", "testIsCreateable", nil)
	if result.Type != TypeBoolean || result.Data.(bool) != false {
		t.Errorf("expected isCreateable() = false for restricted profile, got %v", result)
	}
}

func TestNoPermissionsRowMeansFullAccess(t *testing.T) {
	source := `
@isTest
public class FullAccessTest {
    public static void testFullAccess() {
        User u = new User();
        u.Username = 'noperm@test.com';
        u.ProfileId = '00eNOPERMROW';
        insert u;

        System.runAs(u) {
            Account acc = new Account();
            acc.Name = 'Should Succeed';
            insert as user acc;
        }
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	// No ObjectPermissions rows exist → full access, should not panic
	interp.ExecuteMethod("FullAccessTest", "testFullAccess", nil)
}

func TestInheritedSharingWithRunAs(t *testing.T) {
	classSource := `
public inherited sharing class InheritedTest {
    public static Integer countAccounts() {
        List<Account> accs = [SELECT Id FROM Account];
        return accs.size();
    }
}`
	interp := setupWithRealEngineMulti(t, classSource)

	interp.engine.Insert("Account", []map[string]any{
		{"Name": "Acc1", "OwnerId": "005OWNER1"},
		{"Name": "Acc2", "OwnerId": "005OWNER2"},
	})

	// "inherited sharing" inside runAs → behaves as "with sharing"
	interp.execCtx = &executionContext{
		userID:     "005OWNER1",
		userFields: map[string]*Value{},
	}
	result := interp.ExecuteMethod("InheritedTest", "countAccounts", nil)
	interp.execCtx = nil
	if result.Data.(int) != 1 {
		t.Errorf("inherited sharing + runAs expected 1 record, got %v", result.Data)
	}
}

func TestInheritedSharingWithoutRunAs(t *testing.T) {
	classSource := `
public inherited sharing class InheritedTest2 {
    public static Integer countAccounts() {
        List<Account> accs = [SELECT Id FROM Account];
        return accs.size();
    }
}`
	interp := setupWithRealEngineMulti(t, classSource)

	interp.engine.Insert("Account", []map[string]any{
		{"Name": "Acc1", "OwnerId": "005OWNER1"},
		{"Name": "Acc2", "OwnerId": "005OWNER2"},
	})

	// "inherited sharing" at entry point defaults to "with sharing" per Salesforce docs.
	// However, without a runAs context there is no user to filter by, so all records
	// are visible (sharing filter requires execCtx to be set).
	result := interp.ExecuteMethod("InheritedTest2", "countAccounts", nil)
	if result.Data.(int) != 2 {
		t.Errorf("inherited sharing without runAs expected 2 records (no user to filter by), got %v", result.Data)
	}
}

func TestSecurityEnforcedDoesNotEnforceSharing(t *testing.T) {
	// SECURITY_ENFORCED enforces CRUD+FLS but NOT sharing.
	// A "without sharing" class using SECURITY_ENFORCED should see all records.
	classSource := `
public without sharing class SecEnfTest {
    public static Integer countAccounts() {
        List<Account> accs = [SELECT Id FROM Account WITH SECURITY_ENFORCED];
        return accs.size();
    }
}`
	interp := setupWithRealEngineMulti(t, classSource)

	interp.engine.Insert("Account", []map[string]any{
		{"Name": "Acc1", "OwnerId": "005OWNER1"},
		{"Name": "Acc2", "OwnerId": "005OWNER2"},
	})

	// Inside runAs, SECURITY_ENFORCED should NOT filter by owner
	interp.execCtx = &executionContext{
		userID:     "005OWNER1",
		userFields: map[string]*Value{},
	}
	result := interp.ExecuteMethod("SecEnfTest", "countAccounts", nil)
	interp.execCtx = nil
	if result.Data.(int) != 2 {
		t.Errorf("SECURITY_ENFORCED should not enforce sharing, expected 2 records, got %v", result.Data)
	}
}

func TestUserModeEnforcesSharing(t *testing.T) {
	// USER_MODE enforces CRUD+FLS+sharing regardless of class sharing keyword.
	// Even a "without sharing" class should have sharing enforced with USER_MODE.
	classSource := `
public without sharing class UserModeShareTest {
    public static Integer countAccounts() {
        List<Account> accs = [SELECT Id FROM Account WITH USER_MODE];
        return accs.size();
    }
}`
	interp := setupWithRealEngineMulti(t, classSource)

	interp.engine.Insert("Account", []map[string]any{
		{"Name": "Acc1", "OwnerId": "005OWNER1"},
		{"Name": "Acc2", "OwnerId": "005OWNER2"},
	})

	// USER_MODE should enforce sharing even on "without sharing" class
	interp.execCtx = &executionContext{
		userID:     "005OWNER1",
		userFields: map[string]*Value{},
	}
	result := interp.ExecuteMethod("UserModeShareTest", "countAccounts", nil)
	interp.execCtx = nil
	if result.Data.(int) != 1 {
		t.Errorf("USER_MODE should enforce sharing, expected 1 record, got %v", result.Data)
	}
}

// --- Call-chain sharing inheritance tests ---

func TestInheritedSharingInheritsCallerWithSharing(t *testing.T) {
	// A "with sharing" class calls an "inherited sharing" class.
	// The inherited class should inherit "with sharing" from the caller.
	callerSource := `
public with sharing class Caller {
    public static Integer doQuery() {
        return Callee.countAccounts();
    }
}`
	calleeSource := `
public inherited sharing class Callee {
    public static Integer countAccounts() {
        List<Account> accs = [SELECT Id FROM Account];
        return accs.size();
    }
}`
	interp := setupWithRealEngineMulti(t, callerSource, calleeSource)

	interp.engine.Insert("Account", []map[string]any{
		{"Name": "Acc1", "OwnerId": "005OWNER1"},
		{"Name": "Acc2", "OwnerId": "005OWNER2"},
	})

	interp.execCtx = &executionContext{
		userID:     "005OWNER1",
		userFields: map[string]*Value{},
	}
	result := interp.ExecuteMethod("Caller", "doQuery", nil)
	interp.execCtx = nil
	if result.Data.(int) != 1 {
		t.Errorf("inherited sharing should inherit 'with' from caller, expected 1 record, got %v", result.Data)
	}
}

func TestInheritedSharingInheritsCallerWithoutSharing(t *testing.T) {
	// A "without sharing" class calls an "inherited sharing" class.
	// The inherited class should inherit "without sharing" from the caller.
	callerSource := `
public without sharing class Caller2 {
    public static Integer doQuery() {
        return Callee2.countAccounts();
    }
}`
	calleeSource := `
public inherited sharing class Callee2 {
    public static Integer countAccounts() {
        List<Account> accs = [SELECT Id FROM Account];
        return accs.size();
    }
}`
	interp := setupWithRealEngineMulti(t, callerSource, calleeSource)

	interp.engine.Insert("Account", []map[string]any{
		{"Name": "Acc1", "OwnerId": "005OWNER1"},
		{"Name": "Acc2", "OwnerId": "005OWNER2"},
	})

	interp.execCtx = &executionContext{
		userID:     "005OWNER1",
		userFields: map[string]*Value{},
	}
	result := interp.ExecuteMethod("Caller2", "doQuery", nil)
	interp.execCtx = nil
	if result.Data.(int) != 2 {
		t.Errorf("inherited sharing should inherit 'without' from caller, expected 2 records, got %v", result.Data)
	}
}

func TestOmittedSharingInheritsCallerMode(t *testing.T) {
	// A "with sharing" class calls a class with no sharing modifier.
	// The omitted-modifier class should inherit the caller's sharing mode.
	callerSource := `
public with sharing class SharingCaller {
    public static Integer doQuery() {
        return NoModClass.countAccounts();
    }
}`
	calleeSource := `
public class NoModClass {
    public static Integer countAccounts() {
        List<Account> accs = [SELECT Id FROM Account];
        return accs.size();
    }
}`
	interp := setupWithRealEngineMulti(t, callerSource, calleeSource)

	interp.engine.Insert("Account", []map[string]any{
		{"Name": "Acc1", "OwnerId": "005OWNER1"},
		{"Name": "Acc2", "OwnerId": "005OWNER2"},
	})

	interp.execCtx = &executionContext{
		userID:     "005OWNER1",
		userFields: map[string]*Value{},
	}
	result := interp.ExecuteMethod("SharingCaller", "doQuery", nil)
	interp.execCtx = nil
	if result.Data.(int) != 1 {
		t.Errorf("omitted-modifier class should inherit 'with' from caller, expected 1 record, got %v", result.Data)
	}
}

func TestOmittedSharingAtEntryPointDefaultsWithout(t *testing.T) {
	// A class with no sharing modifier at entry point defaults to "without sharing".
	classSource := `
public class NoModEntry {
    public static Integer countAccounts() {
        List<Account> accs = [SELECT Id FROM Account];
        return accs.size();
    }
}`
	interp := setupWithRealEngineMulti(t, classSource)

	interp.engine.Insert("Account", []map[string]any{
		{"Name": "Acc1", "OwnerId": "005OWNER1"},
		{"Name": "Acc2", "OwnerId": "005OWNER2"},
	})

	interp.execCtx = &executionContext{
		userID:     "005OWNER1",
		userFields: map[string]*Value{},
	}
	result := interp.ExecuteMethod("NoModEntry", "countAccounts", nil)
	interp.execCtx = nil
	if result.Data.(int) != 2 {
		t.Errorf("no-modifier entry point should default to 'without', expected 2 records, got %v", result.Data)
	}
}

// --- Security.stripInaccessible tests ---

func TestStripInaccessibleRemovesFields(t *testing.T) {
	source := `
@isTest
public class StripTest {
    public static Integer testStrip() {
        Profile p = new Profile();
        p.Name = 'StripProfile';
        insert p;

        FieldPermissions fp = new FieldPermissions();
        fp.ParentId = p.Id;
        fp.SobjectType = 'Account';
        fp.Field = 'Account.Industry';
        fp.PermissionsRead = false;
        fp.PermissionsEdit = false;
        insert fp;

        User u = new User();
        u.Username = 'stripuser@test.com';
        u.ProfileId = p.Id;
        insert u;

        List<Account> accs = new List<Account>();
        Account acc = new Account();
        acc.Name = 'Test Corp';
        acc.Industry = 'Tech';
        accs.add(acc);

        SObjectAccessDecision decision;
        System.runAs(u) {
            decision = Security.stripInaccessible(AccessType.READABLE, accs);
        }
        List<Account> stripped = decision.getRecords();
        return stripped.size();
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	result := interp.ExecuteMethod("StripTest", "testStrip", nil)
	if result.Data.(int) != 1 {
		t.Errorf("expected 1 stripped record, got %v", result.Data)
	}
}

func TestStripInaccessibleGetRemovedFields(t *testing.T) {
	source := `
@isTest
public class StripTest2 {
    public static Integer testRemovedFields() {
        Profile p = new Profile();
        p.Name = 'StripProfile2';
        insert p;

        FieldPermissions fp = new FieldPermissions();
        fp.ParentId = p.Id;
        fp.SobjectType = 'Account';
        fp.Field = 'Account.Industry';
        fp.PermissionsRead = false;
        fp.PermissionsEdit = false;
        insert fp;

        User u = new User();
        u.Username = 'stripuser2@test.com';
        u.ProfileId = p.Id;
        insert u;

        List<Account> accs = new List<Account>();
        Account acc = new Account();
        acc.Name = 'Test Corp';
        acc.Industry = 'Tech';
        accs.add(acc);

        SObjectAccessDecision decision;
        System.runAs(u) {
            decision = Security.stripInaccessible(AccessType.READABLE, accs);
        }
        Map<String, Set<String>> removed = decision.getRemovedFields();
        return removed.size();
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	result := interp.ExecuteMethod("StripTest2", "testRemovedFields", nil)
	if result.Data.(int) != 1 {
		t.Errorf("expected 1 SObject type in removedFields, got %v", result.Data)
	}
}

func TestStripInaccessibleNoPermRowAllowsAll(t *testing.T) {
	source := `
@isTest
public class StripTest3 {
    public static Integer testNoRestriction() {
        User u = new User();
        u.Username = 'fullaccess@test.com';
        u.ProfileId = '00eNOROW';
        insert u;

        List<Account> accs = new List<Account>();
        Account acc = new Account();
        acc.Name = 'Test Corp';
        acc.Industry = 'Tech';
        accs.add(acc);

        SObjectAccessDecision decision;
        System.runAs(u) {
            decision = Security.stripInaccessible(AccessType.READABLE, accs);
        }
        List<Account> stripped = decision.getRecords();
        // No FieldPermissions rows → full access, nothing stripped
        Account first = stripped[0];
        return first.Industry == 'Tech' ? 1 : 0;
    }
}`
	interp := setupWithRealEngineMulti(t, source)
	result := interp.ExecuteMethod("StripTest3", "testNoRestriction", nil)
	if result.Data.(int) != 1 {
		t.Errorf("expected Industry field to be preserved (no restriction), got %v", result.Data)
	}
}

func TestStripInaccessibleCRUDCheck(t *testing.T) {
	source := `
@isTest
public class StripCrudTest {
    public static void testCrudEnforced() {
        Profile p = new Profile();
        p.Name = 'NoCrudProfile';
        insert p;

        ObjectPermissions op = new ObjectPermissions();
        op.ParentId = p.Id;
        op.SobjectType = 'Account';
        op.PermissionsCreate = false;
        op.PermissionsRead = false;
        op.PermissionsEdit = false;
        op.PermissionsDelete = false;
        insert op;

        User u = new User();
        u.Username = 'nocrud@test.com';
        u.ProfileId = p.Id;
        insert u;

        List<Account> accs = new List<Account>();
        accs.add(new Account(Name = 'Test'));

        System.runAs(u) {
            SObjectAccessDecision decision = Security.stripInaccessible(AccessType.READABLE, accs);
        }
    }
}`
	interp := setupWithRealEngineMulti(t, source)

	// stripInaccessible with enforceRootObjectCRUD=true (default) should throw
	// when the profile has no read permission on the object
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected stripInaccessible to throw on CRUD violation")
			return
		}
		ts, ok := r.(*ThrowSignal)
		if !ok {
			t.Errorf("expected ThrowSignal, got %T: %v", r, r)
		}
		_ = ts
	}()
	interp.ExecuteMethod("StripCrudTest", "testCrudEnforced", nil)
}
