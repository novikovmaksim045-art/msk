using System;
using System.Linq;

namespace ChestnyZnakScanner
{
    public static class MarkingCodeParser
    {
        public const char GS = (char)29;

        public static MarkingCode Parse(string input, string source = "", int page = 0)
        {
            var result = new MarkingCode { Source = source, Page = page };
            if (input == null)
            {
                result.Error = "Пустой код";
                return result;
            }

            string s = Normalize(input);
            result.Raw = s;

            if (!s.StartsWith("01", StringComparison.Ordinal))
            {
                result.Error = "Нет AI 01 в начале";
                return result;
            }
            if (s.Length < 18)
            {
                result.Error = "Код слишком короткий";
                return result;
            }

            string gtin = s.Substring(2, Math.Min(14, s.Length - 2));
            if (gtin.Length != 14 || !gtin.All(char.IsDigit))
            {
                result.Error = "GTIN после AI 01 должен содержать 14 цифр";
                return result;
            }
            result.Gtin = gtin;

            int pos = 16;
            if (s.Length < pos + 2 || s.Substring(pos, 2) != "21")
            {
                result.Error = "После GTIN отсутствует AI 21";
                return result;
            }
            pos += 2;

            int gs1 = s.IndexOf(GS, pos);
            if (gs1 < 0)
            {
                result.Error = "После серийного номера отсутствует реальный GS (ASCII 29)";
                return result;
            }

            result.Serial = s.Substring(pos, gs1 - pos);
            if (result.Serial.Length == 0)
            {
                result.Error = "Пустой серийный номер AI 21";
                return result;
            }

            pos = gs1 + 1;
            if (s.Length < pos + 2 || s.Substring(pos, 2) != "91")
            {
                result.Error = "После первого GS отсутствует AI 91";
                return result;
            }
            pos += 2;

            int gs2 = s.IndexOf(GS, pos);
            if (gs2 < 0)
            {
                result.Error = "После AI 91 отсутствует второй GS (ASCII 29)";
                return result;
            }

            result.VerificationKey = s.Substring(pos, gs2 - pos);
            if (result.VerificationKey.Length != 4)
            {
                result.Error = "Поле AI 91 должно содержать 4 символа";
                return result;
            }

            pos = gs2 + 1;
            if (s.Length < pos + 2 || s.Substring(pos, 2) != "92")
            {
                result.Error = "После второго GS отсутствует AI 92";
                return result;
            }
            pos += 2;

            result.Crypto = s.Substring(pos);
            if (result.Crypto.Length != 44)
            {
                result.Error = "Крипточасть AI 92 должна содержать 44 символа; получено: " + result.Crypto.Length;
                return result;
            }

            if (s.IndexOf('\r') >= 0 || s.IndexOf('\n') >= 0 || s.IndexOf('\0') >= 0)
            {
                result.Error = "В коде присутствуют недопустимые управляющие символы";
                return result;
            }

            result.IsValidFullCode = true;
            result.Error = string.Empty;
            return result;
        }

        public static string Normalize(string input)
        {
            string s = input.Trim();
            if (s.StartsWith("]d2", StringComparison.OrdinalIgnoreCase) ||
                s.StartsWith("]d1", StringComparison.OrdinalIgnoreCase))
                s = s.Substring(3);

            if (s.Length > 0 && s[0] == (char)232)
                s = s.Substring(1);

            s = s.Replace("<GS>", GS.ToString())
                 .Replace("[GS]", GS.ToString())
                 .Replace("{GS}", GS.ToString());
            return s;
        }
    }
}