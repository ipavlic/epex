public with sharing class AccountService {
    public static List<Account> getHighValueAccounts(Decimal minRevenue) {
        return [
            SELECT Id, Name, Industry, AnnualRevenue
            FROM Account
            WHERE AnnualRevenue >= :minRevenue
            ORDER BY AnnualRevenue DESC
        ];
    }

    public static Account createAccount(String name, String industry) {
        Account acc = new Account();
        acc.Name = name;
        acc.Industry = industry;
        insert acc;
        return acc;
    }

    public static List<Contact> getAccountContacts(Id accountId) {
        return [
            SELECT Id, FirstName, LastName, Email
            FROM Contact
            WHERE AccountId = :accountId
        ];
    }
}
