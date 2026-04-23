package schema

import "strings"

// StandardObjects returns a map of common Salesforce standard objects
// with their object-specific fields. Standard fields (Id, Name, CreatedDate,
// etc.) are NOT included here — they are added separately by StandardFields().
func StandardObjects() map[string]*SObjectSchema {
	return map[string]*SObjectSchema{
		"Account":                accountSchema(),
		"Contact":                contactSchema(),
		"Lead":                   leadSchema(),
		"Opportunity":            opportunitySchema(),
		"OpportunityLineItem":    opportunityLineItemSchema(),
		"Case":                   caseSchema(),
		"Task":                   taskSchema(),
		"Event":                  eventSchema(),
		"User":                   userSchema(),
		"Profile":                profileSchema(),
		"UserRole":               userRoleSchema(),
		"Campaign":               campaignSchema(),
		"CampaignMember":         campaignMemberSchema(),
		"Contract":               contractSchema(),
		"Order":                  orderSchema(),
		"OrderItem":              orderItemSchema(),
		"Product2":               product2Schema(),
		"Pricebook2":             pricebook2Schema(),
		"PricebookEntry":         pricebookEntrySchema(),
		"Asset":                  assetSchema(),
		"ContentDocument":        contentDocumentSchema(),
		"ContentVersion":         contentVersionSchema(),
		"Attachment":             attachmentSchema(),
		"Note":                   noteSchema(),
		"FeedItem":               feedItemSchema(),
		"EmailMessage":           emailMessageSchema(),
		"Organization":           organizationSchema(),
		"Group":                  groupSchema(),
		"Solution":               solutionSchema(),
		"OpportunityContactRole": opportunityContactRoleSchema(),
		"AccountContactRelation": accountContactRelationSchema(),
		"RecordType":             recordTypeSchema(),
		"ObjectPermissions":      objectPermissionsSchema(),
		"FieldPermissions":       fieldPermissionsSchema(),
	}
}

// IsStandardObject returns true if the given name (case-insensitive) is
// a known Salesforce standard object.
func IsStandardObject(name string) bool {
	lower := strings.ToLower(name)
	for k := range StandardObjects() {
		if strings.ToLower(k) == lower {
			return true
		}
	}
	return false
}

// GetStandardObject returns a copy of the named standard object schema with
// the universal standard fields already merged in. Returns nil if the object
// is not a known standard object. The lookup is case-insensitive.
func GetStandardObject(name string) *SObjectSchema {
	lower := strings.ToLower(name)
	objects := StandardObjects()
	for k, obj := range objects {
		if strings.ToLower(k) == lower {
			// Build a new schema with standard fields merged in.
			merged := &SObjectSchema{
				Name:   obj.Name,
				Label:  obj.Label,
				Fields: make(map[string]*SObjectField, len(obj.Fields)+8),
			}
			// Add universal standard fields first.
			for fn, fv := range StandardFields() {
				copy := *fv
				merged.Fields[fn] = &copy
			}
			// Layer on object-specific fields (may override standard fields).
			for fn, fv := range obj.Fields {
				copy := *fv
				merged.Fields[fn] = &copy
			}
			return merged
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Account
// ---------------------------------------------------------------------------

func accountSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Account",
		Label: "Account",
		Fields: map[string]*SObjectField{
			// Classification
			"Type":          {FullName: "Type", Type: FieldTypePicklist},
			"Industry":      {FullName: "Industry", Type: FieldTypePicklist},
			"Rating":        {FullName: "Rating", Type: FieldTypePicklist},
			"AccountSource": {FullName: "AccountSource", Type: FieldTypePicklist},
			"Ownership":     {FullName: "Ownership", Type: FieldTypePicklist},
			"CleanStatus":   {FullName: "CleanStatus", Type: FieldTypePicklist},

			// Hierarchy
			"ParentId":       {FullName: "ParentId", Type: FieldTypeLookup, ReferenceTo: "Account"},
			"MasterRecordId": {FullName: "MasterRecordId", Type: FieldTypeLookup, ReferenceTo: "Account"},

			// Billing address
			"BillingStreet":     {FullName: "BillingStreet", Type: FieldTypeTextArea},
			"BillingCity":       {FullName: "BillingCity", Type: FieldTypeText},
			"BillingState":      {FullName: "BillingState", Type: FieldTypeText},
			"BillingPostalCode": {FullName: "BillingPostalCode", Type: FieldTypeText},
			"BillingCountry":    {FullName: "BillingCountry", Type: FieldTypeText},
			"BillingLatitude":   {FullName: "BillingLatitude", Type: FieldTypeNumber},
			"BillingLongitude":  {FullName: "BillingLongitude", Type: FieldTypeNumber},

			// Shipping address
			"ShippingStreet":     {FullName: "ShippingStreet", Type: FieldTypeTextArea},
			"ShippingCity":       {FullName: "ShippingCity", Type: FieldTypeText},
			"ShippingState":      {FullName: "ShippingState", Type: FieldTypeText},
			"ShippingPostalCode": {FullName: "ShippingPostalCode", Type: FieldTypeText},
			"ShippingCountry":    {FullName: "ShippingCountry", Type: FieldTypeText},
			"ShippingLatitude":   {FullName: "ShippingLatitude", Type: FieldTypeNumber},
			"ShippingLongitude":  {FullName: "ShippingLongitude", Type: FieldTypeNumber},

			// Contact info
			"Phone": {FullName: "Phone", Type: FieldTypePhone},
			"Fax":   {FullName: "Fax", Type: FieldTypePhone},

			// Company details
			"AccountNumber":    {FullName: "AccountNumber", Type: FieldTypeText},
			"Website":          {FullName: "Website", Type: FieldTypeUrl},
			"Sic":              {FullName: "Sic", Type: FieldTypeText},
			"AnnualRevenue":    {FullName: "AnnualRevenue", Type: FieldTypeCurrency},
			"NumberOfEmployees": {FullName: "NumberOfEmployees", Type: FieldTypeNumber},
			"TickerSymbol":     {FullName: "TickerSymbol", Type: FieldTypeText},
			"Description":      {FullName: "Description", Type: FieldTypeLongTextArea},
			"Site":             {FullName: "Site", Type: FieldTypeText},

			// D&B / Jigsaw
			"DunsNumber":      {FullName: "DunsNumber", Type: FieldTypeText},
			"Tradestyle":      {FullName: "Tradestyle", Type: FieldTypeText},
			"NaicsCode":       {FullName: "NaicsCode", Type: FieldTypeText},
			"NaicsDesc":       {FullName: "NaicsDesc", Type: FieldTypeText},
			"YearStarted":     {FullName: "YearStarted", Type: FieldTypeText},
			"SicDesc":         {FullName: "SicDesc", Type: FieldTypeText},
			"DandbCompanyId":  {FullName: "DandbCompanyId", Type: FieldTypeLookup, ReferenceTo: "DandBCompany"},
			"Jigsaw":          {FullName: "Jigsaw", Type: FieldTypeText},
			"JigsawCompanyId": {FullName: "JigsawCompanyId", Type: FieldTypeText},

			// System / read-only
			"PhotoUrl":          {FullName: "PhotoUrl", Type: FieldTypeUrl},
			"LastActivityDate":  {FullName: "LastActivityDate", Type: FieldTypeDate},
			"LastViewedDate":    {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate": {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
		},
	}
}

// ---------------------------------------------------------------------------
// Contact
// ---------------------------------------------------------------------------

func contactSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Contact",
		Label: "Contact",
		Fields: map[string]*SObjectField{
			// Relationships
			"AccountId":      {FullName: "AccountId", Type: FieldTypeLookup, ReferenceTo: "Account"},
			"ReportsToId":    {FullName: "ReportsToId", Type: FieldTypeLookup, ReferenceTo: "Contact"},
			"IndividualId":   {FullName: "IndividualId", Type: FieldTypeLookup},
			"MasterRecordId": {FullName: "MasterRecordId", Type: FieldTypeLookup, ReferenceTo: "Contact"},

			// Name
			"LastName":   {FullName: "LastName", Type: FieldTypeText},
			"FirstName":  {FullName: "FirstName", Type: FieldTypeText},
			"Salutation": {FullName: "Salutation", Type: FieldTypePicklist},

			// Other address
			"OtherStreet":     {FullName: "OtherStreet", Type: FieldTypeTextArea},
			"OtherCity":       {FullName: "OtherCity", Type: FieldTypeText},
			"OtherState":      {FullName: "OtherState", Type: FieldTypeText},
			"OtherPostalCode": {FullName: "OtherPostalCode", Type: FieldTypeText},
			"OtherCountry":    {FullName: "OtherCountry", Type: FieldTypeText},
			"OtherLatitude":   {FullName: "OtherLatitude", Type: FieldTypeNumber},
			"OtherLongitude":  {FullName: "OtherLongitude", Type: FieldTypeNumber},

			// Mailing address
			"MailingStreet":     {FullName: "MailingStreet", Type: FieldTypeTextArea},
			"MailingCity":       {FullName: "MailingCity", Type: FieldTypeText},
			"MailingState":      {FullName: "MailingState", Type: FieldTypeText},
			"MailingPostalCode": {FullName: "MailingPostalCode", Type: FieldTypeText},
			"MailingCountry":    {FullName: "MailingCountry", Type: FieldTypeText},
			"MailingLatitude":   {FullName: "MailingLatitude", Type: FieldTypeNumber},
			"MailingLongitude":  {FullName: "MailingLongitude", Type: FieldTypeNumber},

			// Phones
			"Phone":          {FullName: "Phone", Type: FieldTypePhone},
			"Fax":            {FullName: "Fax", Type: FieldTypePhone},
			"MobilePhone":    {FullName: "MobilePhone", Type: FieldTypePhone},
			"HomePhone":      {FullName: "HomePhone", Type: FieldTypePhone},
			"OtherPhone":     {FullName: "OtherPhone", Type: FieldTypePhone},
			"AssistantPhone": {FullName: "AssistantPhone", Type: FieldTypePhone},

			// Communication
			"Email":              {FullName: "Email", Type: FieldTypeEmail},
			"HasOptedOutOfEmail": {FullName: "HasOptedOutOfEmail", Type: FieldTypeCheckbox},
			"HasOptedOutOfFax":   {FullName: "HasOptedOutOfFax", Type: FieldTypeCheckbox},
			"DoNotCall":          {FullName: "DoNotCall", Type: FieldTypeCheckbox},

			// Professional
			"Title":         {FullName: "Title", Type: FieldTypeText},
			"Department":    {FullName: "Department", Type: FieldTypeText},
			"AssistantName": {FullName: "AssistantName", Type: FieldTypeText},
			"LeadSource":    {FullName: "LeadSource", Type: FieldTypePicklist},
			"Birthdate":     {FullName: "Birthdate", Type: FieldTypeDate},
			"Description":   {FullName: "Description", Type: FieldTypeLongTextArea},

			// Email bounce
			"EmailBouncedReason": {FullName: "EmailBouncedReason", Type: FieldTypeText},
			"EmailBouncedDate":   {FullName: "EmailBouncedDate", Type: FieldTypeDateTime},
			"IsEmailBounced":     {FullName: "IsEmailBounced", Type: FieldTypeCheckbox},

			// System / read-only
			"PhotoUrl":          {FullName: "PhotoUrl", Type: FieldTypeUrl},
			"LastActivityDate":  {FullName: "LastActivityDate", Type: FieldTypeDate},
			"LastViewedDate":    {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate": {FullName: "LastReferencedDate", Type: FieldTypeDateTime},

			// Jigsaw
			"Jigsaw":          {FullName: "Jigsaw", Type: FieldTypeText},
			"JigsawContactId": {FullName: "JigsawContactId", Type: FieldTypeText},
			"CleanStatus":     {FullName: "CleanStatus", Type: FieldTypePicklist},
		},
	}
}

// ---------------------------------------------------------------------------
// Lead
// ---------------------------------------------------------------------------

func leadSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Lead",
		Label: "Lead",
		Fields: map[string]*SObjectField{
			// Name
			"LastName":   {FullName: "LastName", Type: FieldTypeText},
			"FirstName":  {FullName: "FirstName", Type: FieldTypeText},
			"Salutation": {FullName: "Salutation", Type: FieldTypePicklist},

			// Professional
			"Title":   {FullName: "Title", Type: FieldTypeText},
			"Company": {FullName: "Company", Type: FieldTypeText},

			// Address
			"Street":     {FullName: "Street", Type: FieldTypeTextArea},
			"City":       {FullName: "City", Type: FieldTypeText},
			"State":      {FullName: "State", Type: FieldTypeText},
			"PostalCode": {FullName: "PostalCode", Type: FieldTypeText},
			"Country":    {FullName: "Country", Type: FieldTypeText},
			"Latitude":   {FullName: "Latitude", Type: FieldTypeNumber},
			"Longitude":  {FullName: "Longitude", Type: FieldTypeNumber},

			// Contact info
			"Phone":       {FullName: "Phone", Type: FieldTypePhone},
			"MobilePhone": {FullName: "MobilePhone", Type: FieldTypePhone},
			"Fax":         {FullName: "Fax", Type: FieldTypePhone},
			"Email":       {FullName: "Email", Type: FieldTypeEmail},
			"Website":     {FullName: "Website", Type: FieldTypeUrl},
			"PhotoUrl":    {FullName: "PhotoUrl", Type: FieldTypeUrl},

			// Details
			"Description":       {FullName: "Description", Type: FieldTypeLongTextArea},
			"LeadSource":        {FullName: "LeadSource", Type: FieldTypePicklist},
			"Status":            {FullName: "Status", Type: FieldTypePicklist},
			"Industry":          {FullName: "Industry", Type: FieldTypePicklist},
			"Rating":            {FullName: "Rating", Type: FieldTypePicklist},
			"AnnualRevenue":     {FullName: "AnnualRevenue", Type: FieldTypeCurrency},
			"NumberOfEmployees": {FullName: "NumberOfEmployees", Type: FieldTypeNumber},

			// Conversion
			"IsConverted":            {FullName: "IsConverted", Type: FieldTypeCheckbox},
			"ConvertedDate":          {FullName: "ConvertedDate", Type: FieldTypeDate},
			"ConvertedAccountId":     {FullName: "ConvertedAccountId", Type: FieldTypeLookup, ReferenceTo: "Account"},
			"ConvertedContactId":     {FullName: "ConvertedContactId", Type: FieldTypeLookup, ReferenceTo: "Contact"},
			"ConvertedOpportunityId": {FullName: "ConvertedOpportunityId", Type: FieldTypeLookup, ReferenceTo: "Opportunity"},
			"IsUnreadByOwner":        {FullName: "IsUnreadByOwner", Type: FieldTypeCheckbox},

			// System / read-only
			"LastActivityDate":  {FullName: "LastActivityDate", Type: FieldTypeDate},
			"LastViewedDate":    {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate": {FullName: "LastReferencedDate", Type: FieldTypeDateTime},

			// Jigsaw / D&B
			"Jigsaw":            {FullName: "Jigsaw", Type: FieldTypeText},
			"JigsawContactId":   {FullName: "JigsawContactId", Type: FieldTypeText},
			"CleanStatus":       {FullName: "CleanStatus", Type: FieldTypePicklist},
			"CompanyDunsNumber": {FullName: "CompanyDunsNumber", Type: FieldTypeText},

			// Email bounce
			"EmailBouncedReason": {FullName: "EmailBouncedReason", Type: FieldTypeText},
			"EmailBouncedDate":   {FullName: "EmailBouncedDate", Type: FieldTypeDateTime},

			// Other
			"IndividualId":   {FullName: "IndividualId", Type: FieldTypeLookup},
			"MasterRecordId": {FullName: "MasterRecordId", Type: FieldTypeLookup, ReferenceTo: "Lead"},
		},
	}
}

// ---------------------------------------------------------------------------
// Opportunity
// ---------------------------------------------------------------------------

func opportunitySchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Opportunity",
		Label: "Opportunity",
		Fields: map[string]*SObjectField{
			// Relationships
			"AccountId":  {FullName: "AccountId", Type: FieldTypeLookup, ReferenceTo: "Account"},
			"CampaignId": {FullName: "CampaignId", Type: FieldTypeLookup, ReferenceTo: "Campaign"},
			"ContactId":  {FullName: "ContactId", Type: FieldTypeLookup, ReferenceTo: "Contact"},
			"Pricebook2Id": {FullName: "Pricebook2Id", Type: FieldTypeLookup, ReferenceTo: "Pricebook2"},

			// Core fields
			"Description": {FullName: "Description", Type: FieldTypeLongTextArea},
			"StageName":   {FullName: "StageName", Type: FieldTypePicklist},
			"Amount":      {FullName: "Amount", Type: FieldTypeCurrency},
			"Probability": {FullName: "Probability", Type: FieldTypePercent},
			"ExpectedRevenue":          {FullName: "ExpectedRevenue", Type: FieldTypeCurrency},
			"TotalOpportunityQuantity": {FullName: "TotalOpportunityQuantity", Type: FieldTypeNumber},
			"CloseDate":                {FullName: "CloseDate", Type: FieldTypeDate},
			"Type":                     {FullName: "Type", Type: FieldTypePicklist},
			"NextStep":                 {FullName: "NextStep", Type: FieldTypeText},
			"LeadSource":               {FullName: "LeadSource", Type: FieldTypePicklist},

			// Status
			"IsClosed": {FullName: "IsClosed", Type: FieldTypeCheckbox},
			"IsWon":    {FullName: "IsWon", Type: FieldTypeCheckbox},

			// Forecast
			"ForecastCategory":     {FullName: "ForecastCategory", Type: FieldTypePicklist},
			"ForecastCategoryName": {FullName: "ForecastCategoryName", Type: FieldTypePicklist},

			// Line items
			"HasOpportunityLineItem": {FullName: "HasOpportunityLineItem", Type: FieldTypeCheckbox},

			// System / read-only
			"LastActivityDate":  {FullName: "LastActivityDate", Type: FieldTypeDate},
			"LastViewedDate":    {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate": {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
			"FiscalQuarter":     {FullName: "FiscalQuarter", Type: FieldTypeNumber},
			"FiscalYear":        {FullName: "FiscalYear", Type: FieldTypeNumber},
			"HasOpenActivity":   {FullName: "HasOpenActivity", Type: FieldTypeCheckbox},
			"HasOverdueTask":    {FullName: "HasOverdueTask", Type: FieldTypeCheckbox},
		},
	}
}

// ---------------------------------------------------------------------------
// OpportunityLineItem
// ---------------------------------------------------------------------------

func opportunityLineItemSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "OpportunityLineItem",
		Label: "Opportunity Product",
		Fields: map[string]*SObjectField{
			"OpportunityId":    {FullName: "OpportunityId", Type: FieldTypeLookup, ReferenceTo: "Opportunity"},
			"SortOrder":        {FullName: "SortOrder", Type: FieldTypeNumber},
			"PricebookEntryId": {FullName: "PricebookEntryId", Type: FieldTypeLookup, ReferenceTo: "PricebookEntry"},
			"Product2Id":       {FullName: "Product2Id", Type: FieldTypeLookup, ReferenceTo: "Product2"},
			"ProductCode":      {FullName: "ProductCode", Type: FieldTypeText},
			"Quantity":         {FullName: "Quantity", Type: FieldTypeNumber},
			"TotalPrice":       {FullName: "TotalPrice", Type: FieldTypeCurrency},
			"UnitPrice":        {FullName: "UnitPrice", Type: FieldTypeCurrency},
			"ListPrice":        {FullName: "ListPrice", Type: FieldTypeCurrency},
			"ServiceDate":      {FullName: "ServiceDate", Type: FieldTypeDate},
			"Description":      {FullName: "Description", Type: FieldTypeText},
			"Discount":         {FullName: "Discount", Type: FieldTypePercent},
			"Subtotal":         {FullName: "Subtotal", Type: FieldTypeCurrency},
		},
	}
}

// ---------------------------------------------------------------------------
// Case
// ---------------------------------------------------------------------------

func caseSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Case",
		Label: "Case",
		Fields: map[string]*SObjectField{
			// Relationships
			"ContactId":      {FullName: "ContactId", Type: FieldTypeLookup, ReferenceTo: "Contact"},
			"AccountId":      {FullName: "AccountId", Type: FieldTypeLookup, ReferenceTo: "Account"},
			"AssetId":        {FullName: "AssetId", Type: FieldTypeLookup, ReferenceTo: "Asset"},
			"ParentId":       {FullName: "ParentId", Type: FieldTypeLookup, ReferenceTo: "Case"},
			"MasterRecordId": {FullName: "MasterRecordId", Type: FieldTypeLookup, ReferenceTo: "Case"},

			// Supplied info
			"SuppliedName":    {FullName: "SuppliedName", Type: FieldTypeText},
			"SuppliedEmail":   {FullName: "SuppliedEmail", Type: FieldTypeEmail},
			"SuppliedPhone":   {FullName: "SuppliedPhone", Type: FieldTypeText},
			"SuppliedCompany": {FullName: "SuppliedCompany", Type: FieldTypeText},

			// Classification
			"Type":     {FullName: "Type", Type: FieldTypePicklist},
			"Status":   {FullName: "Status", Type: FieldTypePicklist},
			"Reason":   {FullName: "Reason", Type: FieldTypePicklist},
			"Origin":   {FullName: "Origin", Type: FieldTypePicklist},
			"Priority": {FullName: "Priority", Type: FieldTypePicklist},

			// Details
			"Subject":     {FullName: "Subject", Type: FieldTypeText},
			"Description": {FullName: "Description", Type: FieldTypeLongTextArea},
			"Comments":    {FullName: "Comments", Type: FieldTypeLongTextArea},
			"CaseNumber":  {FullName: "CaseNumber", Type: FieldTypeAutoNumber},

			// Status flags
			"IsClosed":    {FullName: "IsClosed", Type: FieldTypeCheckbox},
			"IsEscalated": {FullName: "IsEscalated", Type: FieldTypeCheckbox},
			"ClosedDate":  {FullName: "ClosedDate", Type: FieldTypeDateTime},

			// Contact info (read-only)
			"ContactPhone":  {FullName: "ContactPhone", Type: FieldTypePhone},
			"ContactMobile": {FullName: "ContactMobile", Type: FieldTypePhone},
			"ContactEmail":  {FullName: "ContactEmail", Type: FieldTypeEmail},
			"ContactFax":    {FullName: "ContactFax", Type: FieldTypePhone},

			// System / read-only
			"LastViewedDate":    {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate": {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
		},
	}
}

// ---------------------------------------------------------------------------
// Task
// ---------------------------------------------------------------------------

func taskSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Task",
		Label: "Task",
		Fields: map[string]*SObjectField{
			// Relationships
			"WhoId":     {FullName: "WhoId", Type: FieldTypeLookup},
			"WhatId":    {FullName: "WhatId", Type: FieldTypeLookup},
			"AccountId": {FullName: "AccountId", Type: FieldTypeLookup, ReferenceTo: "Account"},

			// Core fields
			"Subject":      {FullName: "Subject", Type: FieldTypePicklist},
			"ActivityDate": {FullName: "ActivityDate", Type: FieldTypeDate},
			"Status":       {FullName: "Status", Type: FieldTypePicklist},
			"Priority":     {FullName: "Priority", Type: FieldTypePicklist},
			"IsHighPriority": {FullName: "IsHighPriority", Type: FieldTypeCheckbox},
			"Description":  {FullName: "Description", Type: FieldTypeLongTextArea},

			// Status
			"IsClosed":          {FullName: "IsClosed", Type: FieldTypeCheckbox},
			"IsArchived":        {FullName: "IsArchived", Type: FieldTypeCheckbox},
			"CompletedDateTime": {FullName: "CompletedDateTime", Type: FieldTypeDateTime},

			// Call fields
			"CallDurationInSeconds": {FullName: "CallDurationInSeconds", Type: FieldTypeNumber},
			"CallType":              {FullName: "CallType", Type: FieldTypePicklist},
			"CallDisposition":       {FullName: "CallDisposition", Type: FieldTypeText},
			"CallObject":            {FullName: "CallObject", Type: FieldTypeText},

			// Reminder / recurrence
			"ReminderDateTime": {FullName: "ReminderDateTime", Type: FieldTypeDateTime},
			"IsReminderSet":    {FullName: "IsReminderSet", Type: FieldTypeCheckbox},
			"IsRecurrence":     {FullName: "IsRecurrence", Type: FieldTypeCheckbox},
			"TaskSubtype":      {FullName: "TaskSubtype", Type: FieldTypePicklist},
		},
	}
}

// ---------------------------------------------------------------------------
// Event
// ---------------------------------------------------------------------------

func eventSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Event",
		Label: "Event",
		Fields: map[string]*SObjectField{
			// Relationships
			"WhoId":     {FullName: "WhoId", Type: FieldTypeLookup},
			"WhatId":    {FullName: "WhatId", Type: FieldTypeLookup},
			"AccountId": {FullName: "AccountId", Type: FieldTypeLookup, ReferenceTo: "Account"},

			// Core fields
			"Subject":          {FullName: "Subject", Type: FieldTypeText},
			"Location":         {FullName: "Location", Type: FieldTypeText},
			"IsAllDayEvent":    {FullName: "IsAllDayEvent", Type: FieldTypeCheckbox},
			"ActivityDateTime": {FullName: "ActivityDateTime", Type: FieldTypeDateTime},
			"ActivityDate":     {FullName: "ActivityDate", Type: FieldTypeDate},
			"DurationInMinutes": {FullName: "DurationInMinutes", Type: FieldTypeNumber},
			"StartDateTime":    {FullName: "StartDateTime", Type: FieldTypeDateTime},
			"EndDateTime":      {FullName: "EndDateTime", Type: FieldTypeDateTime},
			"EndDate":          {FullName: "EndDate", Type: FieldTypeDate},
			"Description":      {FullName: "Description", Type: FieldTypeLongTextArea},

			// Flags
			"IsPrivate":    {FullName: "IsPrivate", Type: FieldTypeCheckbox},
			"ShowAs":       {FullName: "ShowAs", Type: FieldTypePicklist},
			"IsChild":      {FullName: "IsChild", Type: FieldTypeCheckbox},
			"IsGroupEvent": {FullName: "IsGroupEvent", Type: FieldTypeCheckbox},
			"GroupEventType": {FullName: "GroupEventType", Type: FieldTypePicklist},
			"IsArchived":   {FullName: "IsArchived", Type: FieldTypeCheckbox},

			// Reminder / recurrence
			"IsRecurrence":     {FullName: "IsRecurrence", Type: FieldTypeCheckbox},
			"ReminderDateTime": {FullName: "ReminderDateTime", Type: FieldTypeDateTime},
			"IsReminderSet":    {FullName: "IsReminderSet", Type: FieldTypeCheckbox},
			"EventSubtype":     {FullName: "EventSubtype", Type: FieldTypePicklist},
		},
	}
}

// ---------------------------------------------------------------------------
// User
// ---------------------------------------------------------------------------

func userSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "User",
		Label: "User",
		Fields: map[string]*SObjectField{
			// Identity
			"Username":          {FullName: "Username", Type: FieldTypeText},
			"LastName":          {FullName: "LastName", Type: FieldTypeText},
			"FirstName":         {FullName: "FirstName", Type: FieldTypeText},
			"Alias":             {FullName: "Alias", Type: FieldTypeText},
			"CommunityNickname": {FullName: "CommunityNickname", Type: FieldTypeText},
			"IsActive":          {FullName: "IsActive", Type: FieldTypeCheckbox},

			// Company
			"CompanyName": {FullName: "CompanyName", Type: FieldTypeText},
			"Division":    {FullName: "Division", Type: FieldTypeText},
			"Department":  {FullName: "Department", Type: FieldTypeText},
			"Title":       {FullName: "Title", Type: FieldTypeText},
			"EmployeeNumber": {FullName: "EmployeeNumber", Type: FieldTypeText},

			// Address
			"Street":     {FullName: "Street", Type: FieldTypeTextArea},
			"City":       {FullName: "City", Type: FieldTypeText},
			"State":      {FullName: "State", Type: FieldTypeText},
			"PostalCode": {FullName: "PostalCode", Type: FieldTypeText},
			"Country":    {FullName: "Country", Type: FieldTypeText},
			"Latitude":   {FullName: "Latitude", Type: FieldTypeNumber},
			"Longitude":  {FullName: "Longitude", Type: FieldTypeNumber},

			// Contact info
			"Email":       {FullName: "Email", Type: FieldTypeEmail},
			"Phone":       {FullName: "Phone", Type: FieldTypePhone},
			"Fax":         {FullName: "Fax", Type: FieldTypePhone},
			"MobilePhone": {FullName: "MobilePhone", Type: FieldTypePhone},

			// Settings
			"TimeZoneSidKey":    {FullName: "TimeZoneSidKey", Type: FieldTypePicklist},
			"LocaleSidKey":      {FullName: "LocaleSidKey", Type: FieldTypePicklist},
			"EmailEncodingKey":  {FullName: "EmailEncodingKey", Type: FieldTypePicklist},
			"LanguageLocaleKey": {FullName: "LanguageLocaleKey", Type: FieldTypePicklist},
			"UserType":          {FullName: "UserType", Type: FieldTypePicklist},

			// Relationships
			"ProfileId":  {FullName: "ProfileId", Type: FieldTypeLookup, ReferenceTo: "Profile"},
			"UserRoleId": {FullName: "UserRoleId", Type: FieldTypeLookup, ReferenceTo: "UserRole"},
			"ManagerId":  {FullName: "ManagerId", Type: FieldTypeLookup, ReferenceTo: "User"},

			// Profile / social
			"LastLoginDate": {FullName: "LastLoginDate", Type: FieldTypeDateTime},
			"AboutMe":       {FullName: "AboutMe", Type: FieldTypeLongTextArea},
			"SmallPhotoUrl":  {FullName: "SmallPhotoUrl", Type: FieldTypeUrl},
			"FullPhotoUrl":   {FullName: "FullPhotoUrl", Type: FieldTypeUrl},

			// Notification preferences
			"DigestFrequency":                    {FullName: "DigestFrequency", Type: FieldTypePicklist},
			"DefaultGroupNotificationFrequency": {FullName: "DefaultGroupNotificationFrequency", Type: FieldTypePicklist},

			// System / read-only
			"LastViewedDate":    {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate": {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
		},
	}
}

// ---------------------------------------------------------------------------
// Profile
// ---------------------------------------------------------------------------

func profileSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Profile",
		Label: "Profile",
		Fields: map[string]*SObjectField{
			"Name":             {FullName: "Name", Type: FieldTypeText},
			"UserType":         {FullName: "UserType", Type: FieldTypePicklist},
			"Description":      {FullName: "Description", Type: FieldTypeText},
			"UserLicenseId":    {FullName: "UserLicenseId", Type: FieldTypeLookup},
			"IsSsoEnabled":     {FullName: "IsSsoEnabled", Type: FieldTypeCheckbox},
			"LastViewedDate":   {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate": {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
		},
	}
}

// ---------------------------------------------------------------------------
// UserRole
// ---------------------------------------------------------------------------

func userRoleSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "UserRole",
		Label: "Role",
		Fields: map[string]*SObjectField{
			"Name":                          {FullName: "Name", Type: FieldTypeText},
			"ParentRoleId":                  {FullName: "ParentRoleId", Type: FieldTypeLookup, ReferenceTo: "UserRole"},
			"RollupDescription":             {FullName: "RollupDescription", Type: FieldTypeText},
			"OpportunityAccessForAccountOwner": {FullName: "OpportunityAccessForAccountOwner", Type: FieldTypePicklist},
			"CaseAccessForAccountOwner":     {FullName: "CaseAccessForAccountOwner", Type: FieldTypePicklist},
			"ContactAccessForAccountOwner":  {FullName: "ContactAccessForAccountOwner", Type: FieldTypePicklist},
			"DeveloperName":                 {FullName: "DeveloperName", Type: FieldTypeText},
			"PortalType":                    {FullName: "PortalType", Type: FieldTypePicklist},
			"MayForecastManagerShare":       {FullName: "MayForecastManagerShare", Type: FieldTypeCheckbox},
		},
	}
}

// ---------------------------------------------------------------------------
// Campaign
// ---------------------------------------------------------------------------

func campaignSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Campaign",
		Label: "Campaign",
		Fields: map[string]*SObjectField{
			// Hierarchy
			"ParentId": {FullName: "ParentId", Type: FieldTypeLookup, ReferenceTo: "Campaign"},

			// Classification
			"Type":   {FullName: "Type", Type: FieldTypePicklist},
			"Status": {FullName: "Status", Type: FieldTypePicklist},

			// Dates
			"StartDate": {FullName: "StartDate", Type: FieldTypeDate},
			"EndDate":   {FullName: "EndDate", Type: FieldTypeDate},

			// Financials
			"ExpectedRevenue": {FullName: "ExpectedRevenue", Type: FieldTypeCurrency},
			"BudgetedCost":    {FullName: "BudgetedCost", Type: FieldTypeCurrency},
			"ActualCost":      {FullName: "ActualCost", Type: FieldTypeCurrency},

			// Metrics
			"ExpectedResponse":          {FullName: "ExpectedResponse", Type: FieldTypePercent},
			"NumberSent":                {FullName: "NumberSent", Type: FieldTypeNumber},
			"IsActive":                  {FullName: "IsActive", Type: FieldTypeCheckbox},
			"Description":               {FullName: "Description", Type: FieldTypeLongTextArea},
			"NumberOfLeads":             {FullName: "NumberOfLeads", Type: FieldTypeNumber},
			"NumberOfConvertedLeads":    {FullName: "NumberOfConvertedLeads", Type: FieldTypeNumber},
			"NumberOfContacts":          {FullName: "NumberOfContacts", Type: FieldTypeNumber},
			"NumberOfResponses":         {FullName: "NumberOfResponses", Type: FieldTypeNumber},
			"NumberOfWonOpportunities":  {FullName: "NumberOfWonOpportunities", Type: FieldTypeNumber},
			"NumberOfOpportunities":     {FullName: "NumberOfOpportunities", Type: FieldTypeNumber},
			"AmountAllOpportunities":    {FullName: "AmountAllOpportunities", Type: FieldTypeCurrency},
			"AmountWonOpportunities":    {FullName: "AmountWonOpportunities", Type: FieldTypeCurrency},

			// System / read-only
			"LastActivityDate":  {FullName: "LastActivityDate", Type: FieldTypeDate},
			"LastViewedDate":    {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate": {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
		},
	}
}

// ---------------------------------------------------------------------------
// CampaignMember
// ---------------------------------------------------------------------------

func campaignMemberSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "CampaignMember",
		Label: "Campaign Member",
		Fields: map[string]*SObjectField{
			// Relationships
			"CampaignId":          {FullName: "CampaignId", Type: FieldTypeLookup, ReferenceTo: "Campaign"},
			"LeadId":              {FullName: "LeadId", Type: FieldTypeLookup, ReferenceTo: "Lead"},
			"ContactId":           {FullName: "ContactId", Type: FieldTypeLookup, ReferenceTo: "Contact"},
			"LeadOrContactId":     {FullName: "LeadOrContactId", Type: FieldTypeLookup},
			"LeadOrContactOwnerId": {FullName: "LeadOrContactOwnerId", Type: FieldTypeLookup},

			// Status
			"Status":             {FullName: "Status", Type: FieldTypePicklist},
			"HasResponded":       {FullName: "HasResponded", Type: FieldTypeCheckbox},
			"FirstRespondedDate": {FullName: "FirstRespondedDate", Type: FieldTypeDate},

			// Contact info (from lead/contact)
			"Title":              {FullName: "Title", Type: FieldTypeText},
			"CompanyOrAccount":   {FullName: "CompanyOrAccount", Type: FieldTypeText},
			"City":               {FullName: "City", Type: FieldTypeText},
			"State":              {FullName: "State", Type: FieldTypeText},
			"Street":             {FullName: "Street", Type: FieldTypeTextArea},
			"PostalCode":         {FullName: "PostalCode", Type: FieldTypeText},
			"Country":            {FullName: "Country", Type: FieldTypeText},
			"Email":              {FullName: "Email", Type: FieldTypeEmail},
			"Phone":              {FullName: "Phone", Type: FieldTypePhone},
			"Fax":                {FullName: "Fax", Type: FieldTypePhone},
			"MobilePhone":        {FullName: "MobilePhone", Type: FieldTypePhone},
			"DoNotCall":          {FullName: "DoNotCall", Type: FieldTypeCheckbox},
			"HasOptedOutOfEmail": {FullName: "HasOptedOutOfEmail", Type: FieldTypeCheckbox},
			"HasOptedOutOfFax":   {FullName: "HasOptedOutOfFax", Type: FieldTypeCheckbox},
		},
	}
}

// ---------------------------------------------------------------------------
// Contract
// ---------------------------------------------------------------------------

func contractSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Contract",
		Label: "Contract",
		Fields: map[string]*SObjectField{
			// Relationships
			"AccountId":        {FullName: "AccountId", Type: FieldTypeLookup, ReferenceTo: "Account"},
			"CompanySignedId":  {FullName: "CompanySignedId", Type: FieldTypeLookup, ReferenceTo: "User"},
			"CustomerSignedId": {FullName: "CustomerSignedId", Type: FieldTypeLookup, ReferenceTo: "Contact"},
			"ActivatedById":    {FullName: "ActivatedById", Type: FieldTypeLookup, ReferenceTo: "User"},
			"Pricebook2Id":     {FullName: "Pricebook2Id", Type: FieldTypeLookup, ReferenceTo: "Pricebook2"},

			// Dates
			"StartDate":          {FullName: "StartDate", Type: FieldTypeDate},
			"EndDate":            {FullName: "EndDate", Type: FieldTypeDate},
			"CompanySignedDate":  {FullName: "CompanySignedDate", Type: FieldTypeDate},
			"CustomerSignedDate": {FullName: "CustomerSignedDate", Type: FieldTypeDate},
			"ActivatedDate":      {FullName: "ActivatedDate", Type: FieldTypeDateTime},
			"LastApprovedDate":   {FullName: "LastApprovedDate", Type: FieldTypeDateTime},

			// Terms
			"OwnerExpirationNotice": {FullName: "OwnerExpirationNotice", Type: FieldTypePicklist},
			"ContractTerm":          {FullName: "ContractTerm", Type: FieldTypeNumber},
			"Status":                {FullName: "Status", Type: FieldTypePicklist},
			"StatusCode":            {FullName: "StatusCode", Type: FieldTypePicklist},
			"CustomerSignedTitle":   {FullName: "CustomerSignedTitle", Type: FieldTypeText},
			"SpecialTerms":          {FullName: "SpecialTerms", Type: FieldTypeLongTextArea},
			"Description":           {FullName: "Description", Type: FieldTypeLongTextArea},
			"ContractNumber":        {FullName: "ContractNumber", Type: FieldTypeAutoNumber},

			// Billing address
			"BillingStreet":     {FullName: "BillingStreet", Type: FieldTypeTextArea},
			"BillingCity":       {FullName: "BillingCity", Type: FieldTypeText},
			"BillingState":      {FullName: "BillingState", Type: FieldTypeText},
			"BillingPostalCode": {FullName: "BillingPostalCode", Type: FieldTypeText},
			"BillingCountry":    {FullName: "BillingCountry", Type: FieldTypeText},
			"BillingLatitude":   {FullName: "BillingLatitude", Type: FieldTypeNumber},
			"BillingLongitude":  {FullName: "BillingLongitude", Type: FieldTypeNumber},

			// System / read-only
			"LastActivityDate":  {FullName: "LastActivityDate", Type: FieldTypeDate},
			"LastViewedDate":    {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate": {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
		},
	}
}

// ---------------------------------------------------------------------------
// Order
// ---------------------------------------------------------------------------

func orderSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Order",
		Label: "Order",
		Fields: map[string]*SObjectField{
			// Relationships
			"ContractId":            {FullName: "ContractId", Type: FieldTypeLookup, ReferenceTo: "Contract"},
			"AccountId":             {FullName: "AccountId", Type: FieldTypeLookup, ReferenceTo: "Account"},
			"Pricebook2Id":          {FullName: "Pricebook2Id", Type: FieldTypeLookup, ReferenceTo: "Pricebook2"},
			"OriginalOrderId":       {FullName: "OriginalOrderId", Type: FieldTypeLookup, ReferenceTo: "Order"},
			"CustomerAuthorizedById": {FullName: "CustomerAuthorizedById", Type: FieldTypeLookup, ReferenceTo: "Contact"},
			"CompanyAuthorizedById": {FullName: "CompanyAuthorizedById", Type: FieldTypeLookup, ReferenceTo: "User"},
			"BillToContactId":       {FullName: "BillToContactId", Type: FieldTypeLookup, ReferenceTo: "Contact"},
			"ShipToContactId":       {FullName: "ShipToContactId", Type: FieldTypeLookup, ReferenceTo: "Contact"},
			"ActivatedById":         {FullName: "ActivatedById", Type: FieldTypeLookup, ReferenceTo: "User"},

			// Dates
			"EffectiveDate":          {FullName: "EffectiveDate", Type: FieldTypeDate},
			"EndDate":                {FullName: "EndDate", Type: FieldTypeDate},
			"CustomerAuthorizedDate": {FullName: "CustomerAuthorizedDate", Type: FieldTypeDate},
			"CompanyAuthorizedDate":  {FullName: "CompanyAuthorizedDate", Type: FieldTypeDate},
			"PoDate":                 {FullName: "PoDate", Type: FieldTypeDate},
			"ActivatedDate":          {FullName: "ActivatedDate", Type: FieldTypeDateTime},

			// Classification
			"IsReductionOrder": {FullName: "IsReductionOrder", Type: FieldTypeCheckbox},
			"Status":           {FullName: "Status", Type: FieldTypePicklist},
			"StatusCode":       {FullName: "StatusCode", Type: FieldTypePicklist},
			"Type":             {FullName: "Type", Type: FieldTypePicklist},
			"Description":      {FullName: "Description", Type: FieldTypeLongTextArea},

			// Billing address
			"BillingStreet":     {FullName: "BillingStreet", Type: FieldTypeTextArea},
			"BillingCity":       {FullName: "BillingCity", Type: FieldTypeText},
			"BillingState":      {FullName: "BillingState", Type: FieldTypeText},
			"BillingPostalCode": {FullName: "BillingPostalCode", Type: FieldTypeText},
			"BillingCountry":    {FullName: "BillingCountry", Type: FieldTypeText},
			"BillingLatitude":   {FullName: "BillingLatitude", Type: FieldTypeNumber},
			"BillingLongitude":  {FullName: "BillingLongitude", Type: FieldTypeNumber},

			// Shipping address
			"ShippingStreet":     {FullName: "ShippingStreet", Type: FieldTypeTextArea},
			"ShippingCity":       {FullName: "ShippingCity", Type: FieldTypeText},
			"ShippingState":      {FullName: "ShippingState", Type: FieldTypeText},
			"ShippingPostalCode": {FullName: "ShippingPostalCode", Type: FieldTypeText},
			"ShippingCountry":    {FullName: "ShippingCountry", Type: FieldTypeText},
			"ShippingLatitude":   {FullName: "ShippingLatitude", Type: FieldTypeNumber},
			"ShippingLongitude":  {FullName: "ShippingLongitude", Type: FieldTypeNumber},

			// Reference numbers
			"PoNumber":             {FullName: "PoNumber", Type: FieldTypeText},
			"OrderReferenceNumber": {FullName: "OrderReferenceNumber", Type: FieldTypeText},
			"OrderNumber":          {FullName: "OrderNumber", Type: FieldTypeAutoNumber},
			"TotalAmount":          {FullName: "TotalAmount", Type: FieldTypeCurrency},

			// System / read-only
			"LastViewedDate":    {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate": {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
		},
	}
}

// ---------------------------------------------------------------------------
// OrderItem
// ---------------------------------------------------------------------------

func orderItemSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "OrderItem",
		Label: "Order Product",
		Fields: map[string]*SObjectField{
			"OrderId":             {FullName: "OrderId", Type: FieldTypeLookup, ReferenceTo: "Order"},
			"PricebookEntryId":    {FullName: "PricebookEntryId", Type: FieldTypeLookup, ReferenceTo: "PricebookEntry"},
			"Product2Id":          {FullName: "Product2Id", Type: FieldTypeLookup, ReferenceTo: "Product2"},
			"OriginalOrderItemId": {FullName: "OriginalOrderItemId", Type: FieldTypeLookup, ReferenceTo: "OrderItem"},
			"AvailableQuantity":   {FullName: "AvailableQuantity", Type: FieldTypeNumber},
			"Quantity":            {FullName: "Quantity", Type: FieldTypeNumber},
			"UnitPrice":           {FullName: "UnitPrice", Type: FieldTypeCurrency},
			"ListPrice":           {FullName: "ListPrice", Type: FieldTypeCurrency},
			"TotalPrice":          {FullName: "TotalPrice", Type: FieldTypeCurrency},
			"ServiceDate":         {FullName: "ServiceDate", Type: FieldTypeDate},
			"EndDate":             {FullName: "EndDate", Type: FieldTypeDate},
			"Description":         {FullName: "Description", Type: FieldTypeText},
			"OrderItemNumber":     {FullName: "OrderItemNumber", Type: FieldTypeAutoNumber},
		},
	}
}

// ---------------------------------------------------------------------------
// Product2
// ---------------------------------------------------------------------------

func product2Schema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Product2",
		Label: "Product",
		Fields: map[string]*SObjectField{
			"ProductCode":          {FullName: "ProductCode", Type: FieldTypeText},
			"Description":          {FullName: "Description", Type: FieldTypeLongTextArea},
			"IsActive":             {FullName: "IsActive", Type: FieldTypeCheckbox},
			"Family":               {FullName: "Family", Type: FieldTypePicklist},
			"QuantityUnitOfMeasure": {FullName: "QuantityUnitOfMeasure", Type: FieldTypePicklist},
			"StockKeepingUnit":     {FullName: "StockKeepingUnit", Type: FieldTypeText},
			"DisplayUrl":           {FullName: "DisplayUrl", Type: FieldTypeUrl},
			"ExternalId":           {FullName: "ExternalId", Type: FieldTypeText},
			"IsArchived":           {FullName: "IsArchived", Type: FieldTypeCheckbox},
			"LastViewedDate":       {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate":   {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
		},
	}
}

// ---------------------------------------------------------------------------
// Pricebook2
// ---------------------------------------------------------------------------

func pricebook2Schema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Pricebook2",
		Label: "Price Book",
		Fields: map[string]*SObjectField{
			"IsActive":           {FullName: "IsActive", Type: FieldTypeCheckbox},
			"IsStandard":         {FullName: "IsStandard", Type: FieldTypeCheckbox},
			"Description":        {FullName: "Description", Type: FieldTypeText},
			"LastViewedDate":     {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate": {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
		},
	}
}

// ---------------------------------------------------------------------------
// PricebookEntry
// ---------------------------------------------------------------------------

func pricebookEntrySchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "PricebookEntry",
		Label: "Price Book Entry",
		Fields: map[string]*SObjectField{
			"Pricebook2Id":     {FullName: "Pricebook2Id", Type: FieldTypeLookup, ReferenceTo: "Pricebook2"},
			"Product2Id":       {FullName: "Product2Id", Type: FieldTypeLookup, ReferenceTo: "Product2"},
			"UnitPrice":        {FullName: "UnitPrice", Type: FieldTypeCurrency},
			"IsActive":         {FullName: "IsActive", Type: FieldTypeCheckbox},
			"UseStandardPrice": {FullName: "UseStandardPrice", Type: FieldTypeCheckbox},
			"ProductCode":      {FullName: "ProductCode", Type: FieldTypeText},
		},
	}
}

// ---------------------------------------------------------------------------
// Asset
// ---------------------------------------------------------------------------

func assetSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Asset",
		Label: "Asset",
		Fields: map[string]*SObjectField{
			// Relationships
			"ContactId":   {FullName: "ContactId", Type: FieldTypeLookup, ReferenceTo: "Contact"},
			"AccountId":   {FullName: "AccountId", Type: FieldTypeLookup, ReferenceTo: "Account"},
			"ParentId":    {FullName: "ParentId", Type: FieldTypeLookup, ReferenceTo: "Asset"},
			"RootAssetId": {FullName: "RootAssetId", Type: FieldTypeLookup, ReferenceTo: "Asset"},
			"Product2Id":  {FullName: "Product2Id", Type: FieldTypeLookup, ReferenceTo: "Product2"},

			// Details
			"ProductCode":        {FullName: "ProductCode", Type: FieldTypeText},
			"IsCompetitorProduct": {FullName: "IsCompetitorProduct", Type: FieldTypeCheckbox},
			"SerialNumber":       {FullName: "SerialNumber", Type: FieldTypeText},
			"InstallDate":        {FullName: "InstallDate", Type: FieldTypeDate},
			"PurchaseDate":       {FullName: "PurchaseDate", Type: FieldTypeDate},
			"UsageEndDate":       {FullName: "UsageEndDate", Type: FieldTypeDate},
			"Status":             {FullName: "Status", Type: FieldTypePicklist},
			"Price":              {FullName: "Price", Type: FieldTypeCurrency},
			"Quantity":           {FullName: "Quantity", Type: FieldTypeNumber},
			"Description":        {FullName: "Description", Type: FieldTypeLongTextArea},
			"IsInternal":         {FullName: "IsInternal", Type: FieldTypeCheckbox},
			"AssetLevel":         {FullName: "AssetLevel", Type: FieldTypeNumber},
			"StockKeepingUnit":   {FullName: "StockKeepingUnit", Type: FieldTypeText},

			// System / read-only
			"LastViewedDate":    {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate": {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
		},
	}
}

// ---------------------------------------------------------------------------
// ContentDocument
// ---------------------------------------------------------------------------

func contentDocumentSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "ContentDocument",
		Label: "Content Document",
		Fields: map[string]*SObjectField{
			"Title":                    {FullName: "Title", Type: FieldTypeText},
			"PublishStatus":            {FullName: "PublishStatus", Type: FieldTypePicklist},
			"LatestPublishedVersionId": {FullName: "LatestPublishedVersionId", Type: FieldTypeLookup, ReferenceTo: "ContentVersion"},
			"Description":              {FullName: "Description", Type: FieldTypeTextArea},
			"ContentSize":              {FullName: "ContentSize", Type: FieldTypeNumber},
			"FileType":                 {FullName: "FileType", Type: FieldTypeText},
			"FileExtension":            {FullName: "FileExtension", Type: FieldTypeText},
			"IsArchived":               {FullName: "IsArchived", Type: FieldTypeCheckbox},
			"ArchivedById":             {FullName: "ArchivedById", Type: FieldTypeLookup, ReferenceTo: "User"},
			"ArchivedDate":             {FullName: "ArchivedDate", Type: FieldTypeDateTime},
			"ContentModifiedDate":      {FullName: "ContentModifiedDate", Type: FieldTypeDateTime},
			"LastViewedDate":           {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate":       {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
		},
	}
}

// ---------------------------------------------------------------------------
// ContentVersion
// ---------------------------------------------------------------------------

func contentVersionSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "ContentVersion",
		Label: "Content Version",
		Fields: map[string]*SObjectField{
			"ContentDocumentId": {FullName: "ContentDocumentId", Type: FieldTypeLookup, ReferenceTo: "ContentDocument"},
			"IsLatest":          {FullName: "IsLatest", Type: FieldTypeCheckbox},
			"ContentUrl":        {FullName: "ContentUrl", Type: FieldTypeUrl},
			"VersionNumber":     {FullName: "VersionNumber", Type: FieldTypeText},
			"Title":             {FullName: "Title", Type: FieldTypeText},
			"Description":       {FullName: "Description", Type: FieldTypeTextArea},
			"ReasonForChange":   {FullName: "ReasonForChange", Type: FieldTypeText},
			"PathOnClient":      {FullName: "PathOnClient", Type: FieldTypeText},
			"FileType":          {FullName: "FileType", Type: FieldTypeText},
			"PublishStatus":     {FullName: "PublishStatus", Type: FieldTypePicklist},
			"ContentSize":       {FullName: "ContentSize", Type: FieldTypeNumber},
			"FileExtension":     {FullName: "FileExtension", Type: FieldTypeText},
			"Origin":            {FullName: "Origin", Type: FieldTypePicklist},
			"ContentLocation":   {FullName: "ContentLocation", Type: FieldTypePicklist},
			"IsMajorVersion":    {FullName: "IsMajorVersion", Type: FieldTypeCheckbox},
		},
	}
}

// ---------------------------------------------------------------------------
// Attachment
// ---------------------------------------------------------------------------

func attachmentSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Attachment",
		Label: "Attachment",
		Fields: map[string]*SObjectField{
			"ParentId":    {FullName: "ParentId", Type: FieldTypeLookup},
			"IsPrivate":   {FullName: "IsPrivate", Type: FieldTypeCheckbox},
			"ContentType": {FullName: "ContentType", Type: FieldTypeText},
			"BodyLength":  {FullName: "BodyLength", Type: FieldTypeNumber},
			"Description": {FullName: "Description", Type: FieldTypeTextArea},
		},
	}
}

// ---------------------------------------------------------------------------
// Note
// ---------------------------------------------------------------------------

func noteSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Note",
		Label: "Note",
		Fields: map[string]*SObjectField{
			"ParentId":  {FullName: "ParentId", Type: FieldTypeLookup},
			"Title":     {FullName: "Title", Type: FieldTypeText},
			"IsPrivate": {FullName: "IsPrivate", Type: FieldTypeCheckbox},
			"Body":      {FullName: "Body", Type: FieldTypeLongTextArea},
		},
	}
}

// ---------------------------------------------------------------------------
// FeedItem
// ---------------------------------------------------------------------------

func feedItemSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "FeedItem",
		Label: "Feed Item",
		Fields: map[string]*SObjectField{
			"ParentId":        {FullName: "ParentId", Type: FieldTypeLookup},
			"Type":            {FullName: "Type", Type: FieldTypePicklist},
			"CommentCount":    {FullName: "CommentCount", Type: FieldTypeNumber},
			"LikeCount":       {FullName: "LikeCount", Type: FieldTypeNumber},
			"Title":           {FullName: "Title", Type: FieldTypeText},
			"Body":            {FullName: "Body", Type: FieldTypeLongTextArea},
			"LinkUrl":         {FullName: "LinkUrl", Type: FieldTypeUrl},
			"IsRichText":      {FullName: "IsRichText", Type: FieldTypeCheckbox},
			"RelatedRecordId": {FullName: "RelatedRecordId", Type: FieldTypeLookup},
			"InsertedById":    {FullName: "InsertedById", Type: FieldTypeLookup, ReferenceTo: "User"},
			"HasContent":      {FullName: "HasContent", Type: FieldTypeCheckbox},
			"HasLink":         {FullName: "HasLink", Type: FieldTypeCheckbox},
			"IsClosed":        {FullName: "IsClosed", Type: FieldTypeCheckbox},
			"Status":          {FullName: "Status", Type: FieldTypePicklist},
		},
	}
}

// ---------------------------------------------------------------------------
// EmailMessage
// ---------------------------------------------------------------------------

func emailMessageSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "EmailMessage",
		Label: "Email Message",
		Fields: map[string]*SObjectField{
			// Relationships
			"ParentId":              {FullName: "ParentId", Type: FieldTypeLookup, ReferenceTo: "Case"},
			"ActivityId":            {FullName: "ActivityId", Type: FieldTypeLookup, ReferenceTo: "Task"},
			"ReplyToEmailMessageId": {FullName: "ReplyToEmailMessageId", Type: FieldTypeLookup, ReferenceTo: "EmailMessage"},
			"RelatedToId":           {FullName: "RelatedToId", Type: FieldTypeLookup},

			// Content
			"TextBody":  {FullName: "TextBody", Type: FieldTypeLongTextArea},
			"HtmlBody":  {FullName: "HtmlBody", Type: FieldTypeHtml},
			"Headers":   {FullName: "Headers", Type: FieldTypeLongTextArea},
			"Subject":   {FullName: "Subject", Type: FieldTypeText},
			"FromName":  {FullName: "FromName", Type: FieldTypeText},
			"FromAddress": {FullName: "FromAddress", Type: FieldTypeEmail},

			// Addresses
			"ToAddress":  {FullName: "ToAddress", Type: FieldTypeText},
			"CcAddress":  {FullName: "CcAddress", Type: FieldTypeText},
			"BccAddress": {FullName: "BccAddress", Type: FieldTypeText},

			// Flags
			"Incoming":            {FullName: "Incoming", Type: FieldTypeCheckbox},
			"HasAttachment":       {FullName: "HasAttachment", Type: FieldTypeCheckbox},
			"IsExternallyVisible": {FullName: "IsExternallyVisible", Type: FieldTypeCheckbox},

			// Status
			"Status":      {FullName: "Status", Type: FieldTypePicklist},
			"MessageDate": {FullName: "MessageDate", Type: FieldTypeDateTime},

			// Identifiers
			"MessageIdentifier": {FullName: "MessageIdentifier", Type: FieldTypeText},
			"ThreadIdentifier":  {FullName: "ThreadIdentifier", Type: FieldTypeText},
		},
	}
}

// ---------------------------------------------------------------------------
// Organization
// ---------------------------------------------------------------------------

func organizationSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Organization",
		Label: "Organization",
		Fields: map[string]*SObjectField{
			// Address
			"Division":   {FullName: "Division", Type: FieldTypeText},
			"Street":     {FullName: "Street", Type: FieldTypeTextArea},
			"City":       {FullName: "City", Type: FieldTypeText},
			"State":      {FullName: "State", Type: FieldTypeText},
			"PostalCode": {FullName: "PostalCode", Type: FieldTypeText},
			"Country":    {FullName: "Country", Type: FieldTypeText},
			"Latitude":   {FullName: "Latitude", Type: FieldTypeNumber},
			"Longitude":  {FullName: "Longitude", Type: FieldTypeNumber},

			// Contact info
			"Phone":          {FullName: "Phone", Type: FieldTypePhone},
			"Fax":            {FullName: "Fax", Type: FieldTypePhone},
			"PrimaryContact": {FullName: "PrimaryContact", Type: FieldTypeText},

			// Locale / settings
			"DefaultLocaleSidKey": {FullName: "DefaultLocaleSidKey", Type: FieldTypePicklist},
			"TimeZoneSidKey":      {FullName: "TimeZoneSidKey", Type: FieldTypePicklist},
			"LanguageLocaleKey":   {FullName: "LanguageLocaleKey", Type: FieldTypePicklist},

			// Fiscal
			"FiscalYearStartMonth":            {FullName: "FiscalYearStartMonth", Type: FieldTypeNumber},
			"UsesStartDateAsFiscalYearName": {FullName: "UsesStartDateAsFiscalYearName", Type: FieldTypeCheckbox},

			// Org info
			"OrganizationType":      {FullName: "OrganizationType", Type: FieldTypePicklist},
			"NamespacePrefix":       {FullName: "NamespacePrefix", Type: FieldTypeText},
			"InstanceName":          {FullName: "InstanceName", Type: FieldTypeText},
			"IsSandbox":             {FullName: "IsSandbox", Type: FieldTypeCheckbox},
			"TrialExpirationDate":   {FullName: "TrialExpirationDate", Type: FieldTypeDateTime},
			"ComplianceBccEmail":    {FullName: "ComplianceBccEmail", Type: FieldTypeEmail},
			"UiSkin":                {FullName: "UiSkin", Type: FieldTypePicklist},
			"SignupCountryIsoCode":  {FullName: "SignupCountryIsoCode", Type: FieldTypeText},
		},
	}
}

// ---------------------------------------------------------------------------
// Group
// ---------------------------------------------------------------------------

func groupSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Group",
		Label: "Group",
		Fields: map[string]*SObjectField{
			"DeveloperName":            {FullName: "DeveloperName", Type: FieldTypeText},
			"RelatedId":                {FullName: "RelatedId", Type: FieldTypeLookup},
			"Type":                     {FullName: "Type", Type: FieldTypePicklist},
			"Email":                    {FullName: "Email", Type: FieldTypeEmail},
			"DoesSendEmailToMembers":   {FullName: "DoesSendEmailToMembers", Type: FieldTypeCheckbox},
			"DoesIncludeBosses":        {FullName: "DoesIncludeBosses", Type: FieldTypeCheckbox},
		},
	}
}

// ---------------------------------------------------------------------------
// Solution
// ---------------------------------------------------------------------------

func solutionSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "Solution",
		Label: "Solution",
		Fields: map[string]*SObjectField{
			"SolutionName":          {FullName: "SolutionName", Type: FieldTypeText},
			"SolutionNumber":        {FullName: "SolutionNumber", Type: FieldTypeAutoNumber},
			"SolutionNote":          {FullName: "SolutionNote", Type: FieldTypeLongTextArea},
			"Status":                {FullName: "Status", Type: FieldTypePicklist},
			"IsPublished":           {FullName: "IsPublished", Type: FieldTypeCheckbox},
			"IsPublishedInPublicKb": {FullName: "IsPublishedInPublicKb", Type: FieldTypeCheckbox},
			"IsHtml":                {FullName: "IsHtml", Type: FieldTypeCheckbox},
			"IsReviewed":            {FullName: "IsReviewed", Type: FieldTypeCheckbox},
			"TimesUsed":             {FullName: "TimesUsed", Type: FieldTypeNumber},
			"LastViewedDate":        {FullName: "LastViewedDate", Type: FieldTypeDateTime},
			"LastReferencedDate":    {FullName: "LastReferencedDate", Type: FieldTypeDateTime},
		},
	}
}

// ---------------------------------------------------------------------------
// OpportunityContactRole
// ---------------------------------------------------------------------------

func opportunityContactRoleSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "OpportunityContactRole",
		Label: "Opportunity Contact Role",
		Fields: map[string]*SObjectField{
			"OpportunityId": {FullName: "OpportunityId", Type: FieldTypeLookup, ReferenceTo: "Opportunity"},
			"ContactId":     {FullName: "ContactId", Type: FieldTypeLookup, ReferenceTo: "Contact"},
			"Role":          {FullName: "Role", Type: FieldTypePicklist},
			"IsPrimary":     {FullName: "IsPrimary", Type: FieldTypeCheckbox},
		},
	}
}

// ---------------------------------------------------------------------------
// AccountContactRelation
// ---------------------------------------------------------------------------

func accountContactRelationSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "AccountContactRelation",
		Label: "Account Contact Relationship",
		Fields: map[string]*SObjectField{
			"AccountId": {FullName: "AccountId", Type: FieldTypeLookup, ReferenceTo: "Account"},
			"ContactId": {FullName: "ContactId", Type: FieldTypeLookup, ReferenceTo: "Contact"},
			"Roles":     {FullName: "Roles", Type: FieldTypeMultiPicklist},
			"IsDirect":  {FullName: "IsDirect", Type: FieldTypeCheckbox},
			"IsActive":  {FullName: "IsActive", Type: FieldTypeCheckbox},
			"StartDate": {FullName: "StartDate", Type: FieldTypeDate},
			"EndDate":   {FullName: "EndDate", Type: FieldTypeDate},
		},
	}
}

// ---------------------------------------------------------------------------
// RecordType
// ---------------------------------------------------------------------------

func recordTypeSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "RecordType",
		Label: "Record Type",
		Fields: map[string]*SObjectField{
			"SobjectType":       {FullName: "SobjectType", Type: FieldTypePicklist},
			"DeveloperName":     {FullName: "DeveloperName", Type: FieldTypeText},
			"Description":       {FullName: "Description", Type: FieldTypeText},
			"BusinessProcessId": {FullName: "BusinessProcessId", Type: FieldTypeLookup},
			"IsActive":          {FullName: "IsActive", Type: FieldTypeCheckbox},
			"IsPersonType":      {FullName: "IsPersonType", Type: FieldTypeCheckbox},
			"NamespacePrefix":   {FullName: "NamespacePrefix", Type: FieldTypeText},
		},
	}
}

// ---------------------------------------------------------------------------
// ObjectPermissions
// ---------------------------------------------------------------------------

func objectPermissionsSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "ObjectPermissions",
		Label: "Object Permissions",
		Fields: map[string]*SObjectField{
			"ParentId":          {FullName: "ParentId", Type: FieldTypeLookup},
			"SobjectType":       {FullName: "SobjectType", Type: FieldTypeText},
			"PermissionsCreate": {FullName: "PermissionsCreate", Type: FieldTypeCheckbox},
			"PermissionsRead":   {FullName: "PermissionsRead", Type: FieldTypeCheckbox},
			"PermissionsEdit":   {FullName: "PermissionsEdit", Type: FieldTypeCheckbox},
			"PermissionsDelete": {FullName: "PermissionsDelete", Type: FieldTypeCheckbox},
		},
	}
}

// ---------------------------------------------------------------------------
// FieldPermissions
// ---------------------------------------------------------------------------

func fieldPermissionsSchema() *SObjectSchema {
	return &SObjectSchema{
		Name:  "FieldPermissions",
		Label: "Field Permissions",
		Fields: map[string]*SObjectField{
			"ParentId":        {FullName: "ParentId", Type: FieldTypeLookup},
			"SobjectType":     {FullName: "SobjectType", Type: FieldTypeText},
			"Field":           {FullName: "Field", Type: FieldTypeText},
			"PermissionsRead": {FullName: "PermissionsRead", Type: FieldTypeCheckbox},
			"PermissionsEdit": {FullName: "PermissionsEdit", Type: FieldTypeCheckbox},
		},
	}
}
