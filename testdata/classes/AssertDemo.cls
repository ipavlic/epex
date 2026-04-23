@isTest
private class AssertDemo {
    @isTest
    static void testArithmetic() {
        Integer a = 17;
        Integer b = 17 * 2;
        Assert.areEqual(34, b);
        Assert.areNotEqual(0, b);
        Assert.isTrue(b > a);
        Assert.isFalse(b < a);
    }

    @isTest
    static void testNullChecks() {
        String s = null;
        Assert.isNull(s);
        s = 'hello';
        Assert.isNotNull(s);
    }

    @isTest
    static void testFailure() {
        Assert.areEqual(1, 2, 'This should fail');
    }
}
