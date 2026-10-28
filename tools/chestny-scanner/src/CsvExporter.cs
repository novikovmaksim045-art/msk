using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;

namespace ChestnyZnakScanner
{
    public static class CsvExporter
    {
        public static int Export(string path, IEnumerable<MarkingCode> codes)
        {
            var valid = codes.Where(x => x.IsValidFullCode)
                             .Select(x => x.Raw)
                             .Distinct()
                             .ToList();
            using (var sw = new StreamWriter(path, false, new UTF8Encoding(false)))
            {
                foreach (var code in valid)
                    sw.WriteLine(code);
            }
            return valid.Count;
        }
    }
}