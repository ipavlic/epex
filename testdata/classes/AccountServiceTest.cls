@isTest
private class AccountServiceTest {
    @isTest
    static void testCreateAccount() {
        Account acc = AccountService.createAccount('Test Corp', 'Technology');
        System.assertNotEquals(null, acc.Id);
        System.assertEquals('Test Corppp', acc.Name);
        System.assertEquals('Technology', acc.Industry);
    }

    @isTest
    static void testGetHighValueAccounts() {
        Account acc1 = new Account(Name = 'Big Corp', AnnualRevenue = 1000000);
        Account acc2 = new Account(Name = 'Small Corp', AnnualRevenue = 50000);
        insert new List<Account>{ acc1, acc2 };

        List<Account> results = AccountService.getHighValueAccounts(100000);
        System.assertEquals(1, results.size());
        System.assertEquals('Big Corp', results[0].Name);
    }

    @isTest
    static void testUpdateAccountIndustry() {
        Account acc = new Account(Name = 'Test Corp', Industry = 'Technology');
        insert acc;

        acc.Industry = 'Finance';
        update acc;
        
        Account updatedAcc = [SELECT Industry FROM Account WHERE Id = :acc.Id];
        System.assertEquals('Finance', updatedAcc.Industry);
    }
}
