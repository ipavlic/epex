package schema

import "strings"

// FieldType represents a Salesforce field type.
type FieldType string

const (
	FieldTypeAutoNumber       FieldType = "AutoNumber"
	FieldTypeCheckbox         FieldType = "Checkbox"
	FieldTypeCurrency         FieldType = "Currency"
	FieldTypeDate             FieldType = "Date"
	FieldTypeDateTime         FieldType = "DateTime"
	FieldTypeEmail            FieldType = "Email"
	FieldTypeEncryptedText    FieldType = "EncryptedText"
	FieldTypeFormula          FieldType = "Formula"
	FieldTypeHtml             FieldType = "Html"
	FieldTypeLongTextArea     FieldType = "LongTextArea"
	FieldTypeLookup           FieldType = "Lookup"
	FieldTypeMasterDetail     FieldType = "MasterDetail"
	FieldTypeMultiPicklist    FieldType = "MultiselectPicklist"
	FieldTypeNumber           FieldType = "Number"
	FieldTypePercent          FieldType = "Percent"
	FieldTypePhone            FieldType = "Phone"
	FieldTypePicklist         FieldType = "Picklist"
	FieldTypeRollupSummary    FieldType = "Summary"
	FieldTypeText             FieldType = "Text"
	FieldTypeTextArea         FieldType = "TextArea"
	FieldTypeTime             FieldType = "Time"
	FieldTypeUrl              FieldType = "Url"
	FieldTypeId               FieldType = "Id"
)

// SObjectField represents a single field on an SObject.
type SObjectField struct {
	FullName              string
	Label                 string
	Type                  FieldType
	Required              bool
	Unique                bool
	ExternalId            bool
	Length                int
	Precision             int
	Scale                 int
	ReferenceTo           string // For Lookup/MasterDetail: the target SObject
	RelationshipName      string // For Lookup/MasterDetail: parent relationship name (e.g. "Account" for AccountId)
	ChildRelationshipName string // For Lookup/MasterDetail: child relationship name (e.g. "Contacts")
	DefaultValue          string
}

// SObjectSchema represents the metadata for a single SObject.
type SObjectSchema struct {
	Name             string
	Label            string
	PluralLabel      string
	DeploymentStatus string
	SharingModel     string
	NameFieldLabel   string
	NameFieldType    string
	Fields           map[string]*SObjectField // keyed by API name
}

// Schema holds all SObject schemas for a project.
type Schema struct {
	SObjects map[string]*SObjectSchema // keyed by SObject API name
}

// NewSchema creates an empty Schema.
func NewSchema() *Schema {
	return &Schema{SObjects: make(map[string]*SObjectSchema)}
}

// StandardFields returns the standard fields present on every SObject.
func StandardFields() map[string]*SObjectField {
	return map[string]*SObjectField{
		"Id": {
			FullName: "Id",
			Label:    "Record ID",
			Type:     FieldTypeId,
		},
		"Name": {
			FullName: "Name",
			Label:    "Name",
			Type:     FieldTypeText,
			Length:   255,
		},
		"CreatedDate": {
			FullName: "CreatedDate",
			Label:    "Created Date",
			Type:     FieldTypeDateTime,
		},
		"LastModifiedDate": {
			FullName: "LastModifiedDate",
			Label:    "Last Modified Date",
			Type:     FieldTypeDateTime,
		},
		"CreatedById": {
			FullName:         "CreatedById",
			Label:            "Created By ID",
			Type:             FieldTypeLookup,
			ReferenceTo:      "User",
			RelationshipName: "CreatedBy",
		},
		"LastModifiedById": {
			FullName:         "LastModifiedById",
			Label:            "Last Modified By ID",
			Type:             FieldTypeLookup,
			ReferenceTo:      "User",
			RelationshipName: "LastModifiedBy",
		},
		"OwnerId": {
			FullName:         "OwnerId",
			Label:            "Owner ID",
			Type:             FieldTypeLookup,
			ReferenceTo:      "User",
			RelationshipName: "Owner",
		},
		"IsDeleted": {
			FullName:     "IsDeleted",
			Label:        "Deleted",
			Type:         FieldTypeCheckbox,
			DefaultValue: "false",
		},
	}
}

// ParentRelationship holds the resolved metadata for a parent relationship field.
type ParentRelationship struct {
	FKField       string // The foreign key field name on the child (e.g. "AccountId")
	ParentSObject string // The parent SObject name (e.g. "Account")
}

// ChildRelationship holds the resolved metadata for a child relationship.
type ChildRelationship struct {
	ChildSObject string // The child SObject name (e.g. "Contact")
	FKField      string // The foreign key field on the child (e.g. "AccountId")
}

// ResolveParentRelationship resolves a parent relationship name on an SObject
// by matching against the RelationshipName metadata on lookup fields.
func (s *Schema) ResolveParentRelationship(sobjectName, relationshipName string) *ParentRelationship {
	obj := s.FindSObject(sobjectName)
	if obj == nil {
		return nil
	}
	lowerRel := strings.ToLower(relationshipName)

	for _, f := range obj.Fields {
		if (f.Type == FieldTypeLookup || f.Type == FieldTypeMasterDetail) &&
			strings.ToLower(f.RelationshipName) == lowerRel {
			return &ParentRelationship{FKField: f.FullName, ParentSObject: f.ReferenceTo}
		}
	}

	return nil
}

// ResolveChildRelationship resolves a child relationship name on an SObject
// by matching against the ChildRelationshipName metadata on lookup fields
// across all SObjects in the schema.
func (s *Schema) ResolveChildRelationship(sobjectName, relationshipName string) *ChildRelationship {
	lowerRel := strings.ToLower(relationshipName)

	for _, obj := range s.SObjects {
		for _, f := range obj.Fields {
			if (f.Type == FieldTypeLookup || f.Type == FieldTypeMasterDetail) &&
				strings.EqualFold(f.ReferenceTo, sobjectName) &&
				strings.ToLower(f.ChildRelationshipName) == lowerRel {
				return &ChildRelationship{ChildSObject: obj.Name, FKField: f.FullName}
			}
		}
	}

	return nil
}

// FindSObject finds an SObject schema by name (case-insensitive).
func (s *Schema) FindSObject(name string) *SObjectSchema {
	lower := strings.ToLower(name)
	for k, v := range s.SObjects {
		if strings.ToLower(k) == lower {
			return v
		}
	}
	return nil
}

// PopulateRelationshipNames fills in missing RelationshipName and
// ChildRelationshipName on all lookup/master-detail fields using Salesforce's
// deterministic naming rules:
//   - Parent relationship name (RelationshipName): for standard fields, strip
//     the "Id" suffix (AccountId → Account). For custom fields (__c), replace
//     __c with __r (My_Lookup__c → My_Lookup__r).
//   - Child relationship name (ChildRelationshipName): for standard fields,
//     use the well-known Salesforce child relationship name table. For custom
//     fields, the XML metadata provides this directly.
//
// Fields that already have a relationship name set are not overwritten.
func (s *Schema) PopulateRelationshipNames() {
	for _, obj := range s.SObjects {
		for _, f := range obj.Fields {
			if f.Type != FieldTypeLookup && f.Type != FieldTypeMasterDetail {
				continue
			}
			if f.ReferenceTo == "" {
				continue
			}

			// Fill in parent RelationshipName if missing.
			if f.RelationshipName == "" {
				f.RelationshipName = deriveParentRelationshipName(f.FullName)
			}

			// Fill in ChildRelationshipName if missing.
			if f.ChildRelationshipName == "" {
				f.ChildRelationshipName = lookupChildRelationshipName(obj.Name, f.FullName)
			}
		}
	}
}

// deriveParentRelationshipName derives the parent relationship name from a
// lookup field name using Salesforce's naming rules.
func deriveParentRelationshipName(fieldName string) string {
	if strings.HasSuffix(fieldName, "__c") {
		// Custom field: AccountId__c → AccountId__r
		return strings.TrimSuffix(fieldName, "__c") + "__r"
	}
	// Standard field: strip "Id" suffix (AccountId → Account)
	if strings.HasSuffix(fieldName, "Id") {
		return strings.TrimSuffix(fieldName, "Id")
	}
	return fieldName
}

// childRelationshipNames maps (childSObject, fieldName) → child relationship
// name for standard Salesforce objects. This is the authoritative table of
// well-known child relationship names.
var childRelationshipNames = map[[2]string]string{
	// Account relationships
	{"Account", "ParentId"}:       "ChildAccounts",
	{"Account", "MasterRecordId"}: "MergedAccounts",

	// Contact relationships
	{"Contact", "AccountId"}:      "Contacts",
	{"Contact", "ReportsToId"}:    "DirectReports",
	{"Contact", "MasterRecordId"}: "MergedContacts",

	// Lead relationships
	{"Lead", "ConvertedAccountId"}:     "ConvertedLeads",
	{"Lead", "ConvertedContactId"}:     "ConvertedLeads",
	{"Lead", "ConvertedOpportunityId"}: "ConvertedLeads",
	{"Lead", "MasterRecordId"}:         "MergedLeads",

	// Opportunity relationships
	{"Opportunity", "AccountId"}:    "Opportunities",
	{"Opportunity", "CampaignId"}:   "Opportunities",
	{"Opportunity", "ContactId"}:    "Opportunities",
	{"Opportunity", "Pricebook2Id"}: "Opportunities",

	// OpportunityLineItem relationships
	{"OpportunityLineItem", "OpportunityId"}:    "OpportunityLineItems",
	{"OpportunityLineItem", "PricebookEntryId"}: "OpportunityLineItems",
	{"OpportunityLineItem", "Product2Id"}:       "OpportunityLineItems",

	// Case relationships
	{"Case", "ContactId"}:      "Cases",
	{"Case", "AccountId"}:      "Cases",
	{"Case", "AssetId"}:        "Cases",
	{"Case", "ParentId"}:       "ChildCases",
	{"Case", "MasterRecordId"}: "MergedCases",

	// Task relationships
	{"Task", "AccountId"}: "Tasks",

	// Event relationships
	{"Event", "AccountId"}: "Events",

	// User relationships
	{"User", "ProfileId"}:  "Users",
	{"User", "UserRoleId"}: "Users",
	{"User", "ManagerId"}:  "DirectReports",

	// UserRole relationships
	{"UserRole", "ParentRoleId"}: "ChildRoles",

	// Campaign relationships
	{"Campaign", "ParentId"}: "ChildCampaigns",

	// CampaignMember relationships
	{"CampaignMember", "CampaignId"}: "CampaignMembers",
	{"CampaignMember", "LeadId"}:     "CampaignMembers",
	{"CampaignMember", "ContactId"}:  "CampaignMembers",

	// Contract relationships
	{"Contract", "AccountId"}: "Contracts",

	// Order relationships
	{"Order", "ContractId"}:            "Orders",
	{"Order", "AccountId"}:              "Orders",
	{"Order", "Pricebook2Id"}:           "Orders",
	{"Order", "OriginalOrderId"}:        "ChildOrders",

	// OrderItem relationships
	{"OrderItem", "OrderId"}:             "OrderItems",
	{"OrderItem", "PricebookEntryId"}:    "OrderItems",
	{"OrderItem", "Product2Id"}:          "OrderItems",
	{"OrderItem", "OriginalOrderItemId"}: "ChildOrderItems",

	// PricebookEntry relationships
	{"PricebookEntry", "Pricebook2Id"}: "PricebookEntries",
	{"PricebookEntry", "Product2Id"}:   "PricebookEntries",

	// Asset relationships
	{"Asset", "ContactId"}:   "Assets",
	{"Asset", "AccountId"}:   "Assets",
	{"Asset", "ParentId"}:    "ChildAssets",
	{"Asset", "RootAssetId"}: "DescendantAssets",
	{"Asset", "Product2Id"}:  "Assets",

	// ContentVersion relationships
	{"ContentVersion", "ContentDocumentId"}: "ContentVersions",

	// Attachment relationships
	{"Attachment", "ParentId"}: "Attachments",

	// Note relationships
	{"Note", "ParentId"}: "Notes",

	// FeedItem relationships
	{"FeedItem", "ParentId"}: "Feeds",

	// EmailMessage relationships
	{"EmailMessage", "ParentId"}:              "EmailMessages",
	{"EmailMessage", "ActivityId"}:            "EmailMessages",
	{"EmailMessage", "ReplyToEmailMessageId"}: "Replies",

	// OpportunityContactRole relationships
	{"OpportunityContactRole", "OpportunityId"}: "OpportunityContactRoles",
	{"OpportunityContactRole", "ContactId"}:     "OpportunityContactRoles",

	// AccountContactRelation relationships
	{"AccountContactRelation", "AccountId"}: "AccountContactRelations",
	{"AccountContactRelation", "ContactId"}: "AccountContactRelations",
}

// lookupChildRelationshipName returns the child relationship name for a
// standard field, or empty string if not known.
func lookupChildRelationshipName(sobjectName, fieldName string) string {
	return childRelationshipNames[[2]string{sobjectName, fieldName}]
}
