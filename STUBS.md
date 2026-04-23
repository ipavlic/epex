# Stub Return Values

The interpreter stubs certain Salesforce platform APIs with hardcoded values. These are sufficient for most Apex test execution but do not reflect a real org.

## UserInfo

UserInfo methods are context-aware: inside a `System.runAs(user)` block, they return the runAs user's values. Outside runAs, they return defaults.

| Method | Default (outside runAs) | Inside runAs |
|--------|------------------------|--------------|
| `UserInfo.getUserId()` | `"005000000000001"` | User's Id |
| `UserInfo.getUserName()` | `"test@example.com"` | User's Username field |
| `UserInfo.getProfileId()` | `"00e000000000001"` | User's ProfileId field |
| `UserInfo.getOrganizationId()` | `"00D000000000001"` | `"00D000000000001"` |
| `UserInfo.isMultiCurrencyOrganization()` | `false` | `false` |
| `UserInfo.getDefaultCurrency()` | `"USD"` | `"USD"` |
| `UserInfo.getLocale()` | `"en_US"` | `"en_US"` |
| `UserInfo.getTimeZone().getId()` | `"America/Los_Angeles"` | `"America/Los_Angeles"` |

## Test

| Method | Return Value |
|--------|-------------|
| `Test.startTest()` | no-op |
| `Test.stopTest()` | no-op |
| `Test.isRunningTest()` | `true` |
| `Test.getStandardPricebookId()` | `"01s000000000001"` |
| `Test.loadData(type, resource)` | empty `List<SObject>` |

## System

| Method | Return Value |
|--------|-------------|
| `System.debug(msg)` | prints `DEBUG\|<msg>` to stdout |
| `System.assert(condition, [msg])` | throws `AssertException` on failure |
| `System.assertEquals(expected, actual, [msg])` | throws `AssertException` on mismatch |
| `System.assertNotEquals(expected, actual, [msg])` | throws `AssertException` on match |
| `System.abortJob(jobId)` | no-op |
| `System.enqueueJob(queueable)` | `"7071000000000001"` |
| `System.runAs(user) { ... }` | executes block as user (see [User Context](#user-context)) |

## Assert

| Method | Return Value |
|--------|-------------|
| `Assert.areEqual(expected, actual, [msg])` | throws `AssertException` on mismatch |
| `Assert.areNotEqual(expected, actual, [msg])` | throws `AssertException` on match |
| `Assert.isTrue(condition, [msg])` | throws `AssertException` if false |
| `Assert.isFalse(condition, [msg])` | throws `AssertException` if true |
| `Assert.isNull(value, [msg])` | throws `AssertException` if non-null |
| `Assert.isNotNull(value, [msg])` | throws `AssertException` if null |
| `Assert.isInstanceOfType(value, type, [msg])` | throws `AssertException` on type mismatch |
| `Assert.isNotInstanceOfType(value, type, [msg])` | throws `AssertException` on type match |
| `Assert.fail([msg])` | always throws `AssertException` |

## Database

| Method | Return Value |
|--------|-------------|
| `Database.insert(record\|list, [allOrNone])` | `List<Database.SaveResult>` |
| `Database.update(record\|list, [allOrNone])` | `List<Database.SaveResult>` |
| `Database.delete(record\|list, [allOrNone])` | `List<Database.DeleteResult>` |
| `Database.upsert(record\|list, [externalIdField])` | `List<Database.UpsertResult>` |
| `Database.query(soqlString)` | `List<SObject>` |
| `Database.queryWithBinds(soqlString, bindMap, accessLevel)` | `List<SObject>` |

Database methods execute against the in-memory SQLite database, not a stub.

## Type

| Method | Return Value |
|--------|-------------|
| `Type.forName(typeName)` | `System.Type` object |
| `Type.forName(namespace, typeName)` | `System.Type` object (namespace ignored) |
| *instance* `.newInstance()` | new instance of the named type |
| *instance* `.getName()` | the type name as `String` |

`newInstance()` supports primitives (`String`, `Integer`, `Long`, `Double`, `Boolean`), user-defined classes, and SObject types.

## Id

| Method | Return Value |
|--------|-------------|
| `Id.valueOf(value)` | the value as a `String` |
| *instance* `.getSobjectType()` | `Schema.SObjectType` looked up from the ID prefix |

## Schema

| Method | Return Value |
|--------|-------------|
| `Schema.getGlobalDescribe()` | `Map<String, Schema.SObjectType>` from engine schema |
| *SObjectType* `.getDescribe()` | `Schema.DescribeSObjectResult` |
| *SObjectType* `.newSObject()` | empty SObject of that type |
| *DescribeSObjectResult* `.getName()` | SObject API name |
| *DescribeSObjectResult* `.getLabel()` | SObject label |
| *DescribeSObjectResult* `.getKeyPrefix()` | `null` |
| *DescribeSObjectResult* `.isCreateable()` | `true` (permission-aware inside runAs) |
| *DescribeSObjectResult* `.isUpdateable()` | `true` (permission-aware inside runAs) |
| *DescribeSObjectResult* `.isDeletable()` | `true` (permission-aware inside runAs) |
| *DescribeSObjectResult* `.isQueryable()` | `true` (permission-aware inside runAs) |
| *DescribeSObjectResult* `.isAccessible()` | `true` (permission-aware inside runAs) |
| *DescribeSObjectResult* `.isCustom()` | `true` if name ends with `__c` |
| *DescribeSObjectResult* `.fields.getMap()` | `Map<String, Schema.DescribeFieldResult>` |
| *DescribeFieldResult* `.getName()` | field API name |
| *DescribeFieldResult* `.getLabel()` | field label |
| *DescribeFieldResult* `.getType()` | field type as `String` |
| *DescribeFieldResult* `.isRequired()` | from schema definition |
| *DescribeFieldResult* `.isNillable()` | inverse of `isRequired()` |
| *DescribeFieldResult* `.isUnique()` | from schema definition |
| *DescribeFieldResult* `.getLength()` | from schema definition |
| *DescribeFieldResult* `.isCustom()` | `true` if name ends with `__c` |
| *DescribeFieldResult* `.isAccessible()` | `true` (permission-aware inside runAs) |

## Security

| Method | Behavior |
|--------|----------|
| `Security.stripInaccessible(AccessType, List<SObject>)` | Returns `SObjectAccessDecision` with inaccessible fields removed |
| `Security.stripInaccessible(AccessType, List<SObject>, enforceRootCRUD)` | Same, with optional CRUD enforcement toggle |

`AccessType` values: `CREATABLE`, `READABLE`, `UPDATABLE`, `UPSERTABLE`.

`SObjectAccessDecision` methods:

| Method | Return Value |
|--------|-------------|
| `.getRecords()` | `List<SObject>` with inaccessible fields stripped |
| `.getRemovedFields()` | `Map<String, Set<String>>` of removed fields per SObject type |
| `.getModifiedIndexes()` | `Set<Integer>` (stub: empty set) |

## User Context

### System.runAs(user)

`System.runAs(user) { ... }` executes a block in the context of a specific user. Inside the block:
- `UserInfo.getUserId()` returns the user's Id
- `UserInfo.getUserName()` returns the user's Username
- `UserInfo.getProfileId()` returns the user's ProfileId
- INSERT auto-sets `OwnerId` to the running user (when not explicitly set)
- Sharing rules are enforced based on the class keyword and user ownership
- Nested `runAs` blocks are supported; context is restored after each block

### Sharing Keywords

| Keyword | Behavior |
|---------|----------|
| `with sharing` | Enforces sharing: SOQL returns only records owned by the running user |
| `without sharing` | No sharing enforcement: all records visible |
| `inherited sharing` | Inherits caller's sharing mode; defaults to `with sharing` at entry point |
| *(no keyword)* | Inherits caller's sharing mode; defaults to `without sharing` at entry point |

Call-chain inheritance is tracked: when a `with sharing` class calls an `inherited sharing` or no-modifier class, the callee inherits `with sharing`.

Sharing enforcement is a simplified OwnerId-based filter (approximates Salesforce's Private OWD model). Real Salesforce sharing includes role hierarchy, sharing rules, manual sharing, etc.

### DML Access Modes

| Syntax | Behavior |
|--------|----------|
| `INSERT acc;` | System mode (default): no CRUD/FLS checks |
| `INSERT AS USER acc;` | User mode: enforces CRUD via ObjectPermissions |
| `INSERT AS SYSTEM acc;` | Explicit system mode: bypasses all checks |

Same pattern applies to `UPDATE`, `DELETE`, `UPSERT`.

### SOQL Access Modes

| Clause | CRUD | FLS | Sharing | On violation |
|--------|------|-----|---------|-------------|
| *(default)* | No | No | Per class keyword | — |
| `WITH USER_MODE` | Yes | Yes | Yes (always) | Throws exception |
| `WITH SECURITY_ENFORCED` | Yes | Yes | No | Throws exception |
| `WITH SYSTEM_MODE` | No | No | No | — |

### Permissions Model

Permission checks use ObjectPermissions and FieldPermissions SObjects stored in the database. Tests set up permissions by inserting these records:

```apex
// Create a restricted profile
Profile p = new Profile(Name = 'Restricted');
insert p;

// Deny create on Account
ObjectPermissions op = new ObjectPermissions();
op.ParentId = p.Id;
op.SobjectType = 'Account';
op.PermissionsCreate = false;
op.PermissionsRead = true;
insert op;

// Deny read on Account.Industry
FieldPermissions fp = new FieldPermissions();
fp.ParentId = p.Id;
fp.SobjectType = 'Account';
fp.Field = 'Account.Industry';
fp.PermissionsRead = false;
insert fp;

// Create user with that profile
User u = new User(Username = 'test@test.com', ProfileId = p.Id);
insert u;

// Test enforcement
System.runAs(u) {
    insert as user new Account(Name = 'Test'); // throws: no create permission
}
```

**"No permissions row = full access"**: Tests that don't set up ObjectPermissions/FieldPermissions records will see no enforcement. Only tests that explicitly insert permission records will see checks applied.
