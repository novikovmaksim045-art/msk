namespace ChestnyZnakScanner
{
    public sealed class MarkingCode
    {
        public string Raw { get; set; }
        public string Source { get; set; }
        public int Page { get; set; }
        public string Gtin { get; set; }
        public string Serial { get; set; }
        public string VerificationKey { get; set; }
        public string Crypto { get; set; }
        public bool IsValidFullCode { get; set; }
        public string Error { get; set; }
        public string DisplayRaw => (Raw ?? string.Empty).Replace(((char)29).ToString(), "<GS>");
        public string Status => IsValidFullCode ? "OK" : "ОШИБКА";
    }
}