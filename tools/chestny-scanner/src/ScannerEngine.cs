using PdfiumViewer;
using System;
using System.Collections.Generic;
using System.Drawing;
using System.Drawing.Drawing2D;
using System.Drawing.Imaging;
using System.IO;
using System.Linq;
using ZXing;
using ZXing.Common;

namespace ChestnyZnakScanner
{
    public sealed class ScannerEngine
    {
        private readonly BarcodeReader _reader;

        public ScannerEngine()
        {
            _reader = new BarcodeReader
            {
                AutoRotate = true,
                TryInverted = true,
                Options = new DecodingOptions
                {
                    TryHarder = true,
                    PureBarcode = false,
                    PossibleFormats = new List<BarcodeFormat> { BarcodeFormat.DATA_MATRIX }
                }
            };
        }

        public List<MarkingCode> ScanFile(string path, Action<string> progress = null)
        {
            string ext = Path.GetExtension(path).ToLowerInvariant();
            if (ext == ".pdf") return ScanPdf(path, progress);
            if (ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".bmp" || ext == ".tif" || ext == ".tiff")
                return ScanImage(path, progress);
            throw new NotSupportedException("Поддерживаются PDF, PNG, JPG, BMP, TIFF.");
        }

        private List<MarkingCode> ScanPdf(string path, Action<string> progress)
        {
            var list = new List<MarkingCode>();
            using (var pdf = PdfDocument.Load(path))
            {
                for (int i = 0; i < pdf.PageCount; i++)
                {
                    progress?.Invoke($"{Path.GetFileName(path)} — страница {i + 1}/{pdf.PageCount}");
                    int width = (int)Math.Ceiling(pdf.PageSizes[i].Width / 72f * 300f);
                    int height = (int)Math.Ceiling(pdf.PageSizes[i].Height / 72f * 300f);
                    width = Math.Max(900, Math.Min(width, 5000));
                    height = Math.Max(900, Math.Min(height, 5000));
                    using (var img = pdf.Render(i, width, height, 300, 300, PdfRenderFlags.Annotations))
                    using (var bmp = new Bitmap(img))
                    {
                        foreach (var text in DecodeWithFallbacks(bmp))
                            list.Add(MarkingCodeParser.Parse(text, Path.GetFileName(path), i + 1));
                    }
                }
            }
            return Deduplicate(list);
        }

        private List<MarkingCode> ScanImage(string path, Action<string> progress)
        {
            progress?.Invoke(Path.GetFileName(path));
            using (var bmp = (Bitmap)Image.FromFile(path))
            {
                return Deduplicate(DecodeWithFallbacks(bmp)
                    .Select(x => MarkingCodeParser.Parse(x, Path.GetFileName(path), 1)).ToList());
            }
        }

        private IEnumerable<string> DecodeWithFallbacks(Bitmap source)
        {
            var seen = new HashSet<string>();
            foreach (var candidate in BuildCandidates(source))
            {
                using (candidate)
                {
                    Result[] results = null;
                    try { results = _reader.DecodeMultiple(candidate); } catch { }
                    if (results != null)
                    {
                        foreach (var r in results)
                            if (!string.IsNullOrEmpty(r?.Text) && seen.Add(r.Text))
                                yield return r.Text;
                    }

                    try
                    {
                        var one = _reader.Decode(candidate);
                        if (!string.IsNullOrEmpty(one?.Text) && seen.Add(one.Text))
                            yield return one.Text;
                    }
                    catch { }

                    foreach (var tile in BuildTiles(candidate))
                    using (tile)
                    {
                        try
                        {
                            var r = _reader.Decode(tile);
                            if (!string.IsNullOrEmpty(r?.Text) && seen.Add(r.Text))
                                yield return r.Text;
                        }
                        catch { }
                    }
                }
            }
        }

        private static IEnumerable<Bitmap> BuildTiles(Bitmap source)
        {
            foreach (var grid in new[] { new[] { 2, 2 }, new[] { 1, 3 }, new[] { 3, 1 }, new[] { 2, 4 }, new[] { 4, 2 } })
            {
                int cols = grid[0], rows = grid[1];
                int cellW = (int)Math.Ceiling(source.Width / (double)cols);
                int cellH = (int)Math.Ceiling(source.Height / (double)rows);
                int overlapX = Math.Max(12, (int)(cellW * 0.18));
                int overlapY = Math.Max(12, (int)(cellH * 0.18));
                for (int row = 0; row < rows; row++)
                for (int col = 0; col < cols; col++)
                {
                    int x = Math.Max(0, col * cellW - overlapX);
                    int y = Math.Max(0, row * cellH - overlapY);
                    int right = Math.Min(source.Width, (col + 1) * cellW + overlapX);
                    int bottom = Math.Min(source.Height, (row + 1) * cellH + overlapY);
                    var rect = Rectangle.FromLTRB(x, y, right, bottom);
                    if (rect.Width >= 80 && rect.Height >= 80)
                        yield return source.Clone(rect, PixelFormat.Format24bppRgb);
                }
            }
        }

        private IEnumerable<Bitmap> BuildCandidates(Bitmap source)
        {
            yield return new Bitmap(source);
            int max = Math.Max(source.Width, source.Height);
            if (max < 2500)
            {
                double k = Math.Min(2.0, 3000.0 / Math.Max(1, max));
                if (k > 1.15)
                    yield return Resize(source, (int)(source.Width * k), (int)(source.Height * k));
            }
            yield return ToHighContrast(source);
        }

        private static Bitmap Resize(Bitmap source, int width, int height)
        {
            var b = new Bitmap(width, height, PixelFormat.Format24bppRgb);
            using (var g = Graphics.FromImage(b))
            {
                g.InterpolationMode = InterpolationMode.NearestNeighbor;
                g.PixelOffsetMode = PixelOffsetMode.HighQuality;
                g.DrawImage(source, 0, 0, width, height);
            }
            return b;
        }

        private static Bitmap ToHighContrast(Bitmap source)
        {
            var b = new Bitmap(source.Width, source.Height, PixelFormat.Format24bppRgb);
            using (var g = Graphics.FromImage(b))
            {
                var cm = new ColorMatrix(new[]
                {
                    new float[] {1.7f,0,0,0,0}, new float[] {0,1.7f,0,0,0},
                    new float[] {0,0,1.7f,0,0,0}, new float[] {0,0,0,1,0},
                    new float[] {-0.35f,-0.35f,-0.35f,0,1}
                });
                using (var ia = new ImageAttributes())
                {
                    ia.SetColorMatrix(cm);
                    g.DrawImage(source, new Rectangle(0, 0, b.Width, b.Height), 0, 0, source.Width, source.Height, GraphicsUnit.Pixel, ia);
                }
            }
            return b;
        }

        private static List<MarkingCode> Deduplicate(List<MarkingCode> input)
        {
            var seen = new HashSet<string>();
            var output = new List<MarkingCode>();
            foreach (var x in input)
            {
                string key = x.Raw ?? (x.Source + ":" + x.Page + ":" + x.Error);
                if (seen.Add(key)) output.Add(x);
            }
            return output;
        }
    }
}