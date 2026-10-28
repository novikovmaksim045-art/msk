using System;
using System.Collections.Generic;
using System.Drawing;
using System.IO;
using System.Linq;
using System.Threading.Tasks;
using System.Windows.Forms;

namespace ChestnyZnakScanner
{
    public sealed class MainForm : Form
    {
        private readonly ScannerEngine _scanner = new ScannerEngine();
        private readonly List<MarkingCode> _codes = new List<MarkingCode>();
        private readonly DataGridView _grid = new DataGridView();
        private readonly Label _drop = new Label();
        private readonly Label _summary = new Label();
        private readonly ToolStripStatusLabel _status = new ToolStripStatusLabel("Готово");
        private readonly Button _open = new Button();
        private readonly Button _paste = new Button();
        private readonly Button _export = new Button();
        private readonly Button _clear = new Button();
        private bool _busy;

        public MainForm()
        {
            Text = "Честный знак — сканер КМ";
            Width = 1180;
            Height = 720;
            MinimumSize = new Size(900, 560);
            StartPosition = FormStartPosition.CenterScreen;
            Font = new Font("Segoe UI", 9F);
            AllowDrop = true;
            BackColor = Color.White;

            var header = new Panel { Dock = DockStyle.Top, Height = 88, Padding = new Padding(18, 14, 18, 10) };
            var title = new Label { Text = "Сканер кодов маркировки «Честный знак»", AutoSize = true, Font = new Font("Segoe UI Semibold", 16F), Location = new Point(18, 10) };
            var sub = new Label { Text = "PDF / PNG / JPG → полный КМ → CSV для отчёта о нанесении", AutoSize = true, ForeColor = Color.DimGray, Location = new Point(20, 47) };
            header.Controls.Add(title); header.Controls.Add(sub);

            var buttons = new FlowLayoutPanel { Dock = DockStyle.Top, Height = 52, Padding = new Padding(14, 5, 14, 5), FlowDirection = FlowDirection.LeftToRight };
            ConfigureButton(_open, "Открыть файлы", 125);
            ConfigureButton(_paste, "Вставить из сканера", 155);
            ConfigureButton(_export, "Сохранить CSV", 135);
            ConfigureButton(_clear, "Очистить", 95);
            _export.Enabled = false;
            buttons.Controls.AddRange(new Control[] { _open, _paste, _export, _clear });

            _drop.Dock = DockStyle.Top;
            _drop.Height = 64;
            _drop.TextAlign = ContentAlignment.MiddleCenter;
            _drop.Text = "Перетащите сюда PDF или изображения с DataMatrix";
            _drop.BorderStyle = BorderStyle.FixedSingle;
            _drop.ForeColor = Color.DimGray;
            _drop.AllowDrop = true;

            _summary.Dock = DockStyle.Bottom;
            _summary.Height = 34;
            _summary.TextAlign = ContentAlignment.MiddleLeft;
            _summary.Padding = new Padding(10, 0, 0, 0);
            _summary.Text = "Кодов: 0";

            ConfigureGrid();

            var status = new StatusStrip();
            status.Items.Add(_status);

            Controls.Add(_grid);
            Controls.Add(_summary);
            Controls.Add(_drop);
            Controls.Add(buttons);
            Controls.Add(header);
            Controls.Add(status);

            _open.Click += async (s, e) => await OpenFiles();
            _paste.Click += (s, e) => PasteCode();
            _export.Click += (s, e) => ExportCsv();
            _clear.Click += (s, e) => { _codes.Clear(); RefreshGrid(); };
            DragEnter += OnDragEnter; DragDrop += OnDragDrop;
            _drop.DragEnter += OnDragEnter; _drop.DragDrop += OnDragDrop;
        }

        private static void ConfigureButton(Button b, string text, int width)
        {
            b.Text = text; b.Width = width; b.Height = 34; b.FlatStyle = FlatStyle.System;
        }

        private void ConfigureGrid()
        {
            _grid.Dock = DockStyle.Fill;
            _grid.ReadOnly = true;
            _grid.AllowUserToAddRows = false;
            _grid.AllowUserToDeleteRows = false;
            _grid.SelectionMode = DataGridViewSelectionMode.FullRowSelect;
            _grid.MultiSelect = true;
            _grid.AutoGenerateColumns = false;
            _grid.RowHeadersVisible = false;
            _grid.BackgroundColor = Color.White;
            _grid.BorderStyle = BorderStyle.None;
            _grid.AutoSizeRowsMode = DataGridViewAutoSizeRowsMode.AllCells;

            _grid.Columns.Add(new DataGridViewTextBoxColumn { HeaderText = "Статус", DataPropertyName = "Status", Width = 70 });
            _grid.Columns.Add(new DataGridViewTextBoxColumn { HeaderText = "Файл", DataPropertyName = "Source", Width = 180 });
            _grid.Columns.Add(new DataGridViewTextBoxColumn { HeaderText = "Стр.", DataPropertyName = "Page", Width = 45 });
            _grid.Columns.Add(new DataGridViewTextBoxColumn { HeaderText = "GTIN", DataPropertyName = "Gtin", Width = 125 });
            _grid.Columns.Add(new DataGridViewTextBoxColumn { HeaderText = "Серийный № (21)", DataPropertyName = "Serial", Width = 145 });
            _grid.Columns.Add(new DataGridViewTextBoxColumn { HeaderText = "91", DataPropertyName = "VerificationKey", Width = 60 });
            _grid.Columns.Add(new DataGridViewTextBoxColumn { HeaderText = "Крипто 92", DataPropertyName = "Crypto", Width = 260 });
            _grid.Columns.Add(new DataGridViewTextBoxColumn { HeaderText = "Ошибка", DataPropertyName = "Error", AutoSizeMode = DataGridViewAutoSizeColumnMode.Fill, MinimumWidth = 160 });
            _grid.CellFormatting += (s, e) =>
            {
                if (e.RowIndex < 0) return;
                var code = _grid.Rows[e.RowIndex].DataBoundItem as MarkingCode;
                if (code == null) return;
                _grid.Rows[e.RowIndex].DefaultCellStyle.BackColor = code.IsValidFullCode ? Color.Honeydew : Color.MistyRose;
            };
        }

        private async Task OpenFiles()
        {
            using (var d = new OpenFileDialog())
            {
                d.Filter = "PDF и изображения|*.pdf;*.png;*.jpg;*.jpeg;*.bmp;*.tif;*.tiff|Все файлы|*.*";
                d.Multiselect = true;
                if (d.ShowDialog(this) == DialogResult.OK)
                    await ProcessFiles(d.FileNames);
            }
        }

        private void OnDragEnter(object sender, DragEventArgs e)
        {
            if (!_busy && e.Data.GetDataPresent(DataFormats.FileDrop)) e.Effect = DragDropEffects.Copy;
        }

        private async void OnDragDrop(object sender, DragEventArgs e)
        {
            var files = e.Data.GetData(DataFormats.FileDrop) as string[];
            if (files != null) await ProcessFiles(files);
        }

        private async Task ProcessFiles(IEnumerable<string> files)
        {
            if (_busy) return;
            var paths = files.Where(File.Exists).ToArray();
            if (paths.Length == 0) return;
            SetBusy(true);
            try
            {
                foreach (var path in paths)
                {
                    _status.Text = "Сканирование: " + Path.GetFileName(path);
                    var found = await Task.Run(() => _scanner.ScanFile(path, msg => BeginInvoke(new Action(() => _status.Text = msg))));
                    Merge(found);
                    RefreshGrid();
                }
                _status.Text = "Готово";
            }
            catch (Exception ex)
            {
                _status.Text = "Ошибка";
                MessageBox.Show(this, ex.Message, "Ошибка обработки", MessageBoxButtons.OK, MessageBoxIcon.Error);
            }
            finally { SetBusy(false); }
        }

        private void PasteCode()
        {
            string text = Clipboard.ContainsText() ? Clipboard.GetText() : string.Empty;
            if (string.IsNullOrWhiteSpace(text))
            {
                using (var f = new PasteForm())
                    if (f.ShowDialog(this) == DialogResult.OK) text = f.CodeText;
            }
            if (string.IsNullOrWhiteSpace(text)) return;

            foreach (var line in text.Replace("\r", "").Split('\n').Where(x => x.Length > 0))
                Merge(new[] { MarkingCodeParser.Parse(line, "Буфер/сканер", 0) });
            RefreshGrid();
        }

        private void Merge(IEnumerable<MarkingCode> codes)
        {
            var existing = new HashSet<string>(_codes.Where(x => x.Raw != null).Select(x => x.Raw));
            foreach (var c in codes)
                if (c.Raw == null || existing.Add(c.Raw)) _codes.Add(c);
        }

        private void RefreshGrid()
        {
            _grid.DataSource = null;
            _grid.DataSource = _codes.ToList();
            int ok = _codes.Count(x => x.IsValidFullCode);
            int bad = _codes.Count - ok;
            _summary.Text = $"Кодов: {_codes.Count}    корректных: {ok}    ошибок: {bad}    уникальных корректных: {_codes.Where(x => x.IsValidFullCode).Select(x => x.Raw).Distinct().Count()}";
            _export.Enabled = ok > 0 && !_busy;
        }

        private void SetBusy(bool busy)
        {
            _busy = busy;
            _open.Enabled = !busy;
            _paste.Enabled = !busy;
            _clear.Enabled = !busy;
            _export.Enabled = !busy && _codes.Any(x => x.IsValidFullCode);
            UseWaitCursor = busy;
        }

        private void ExportCsv()
        {
            using (var d = new SaveFileDialog())
            {
                d.Filter = "CSV для отчёта о нанесении|*.csv";
                d.FileName = "КМ_для_отчета_о_нанесении.csv";
                if (d.ShowDialog(this) != DialogResult.OK) return;
                int count = CsvExporter.Export(d.FileName, _codes);
                _status.Text = $"CSV сохранён: {count} кодов";
            }
        }
    }

    internal sealed class PasteForm : Form
    {
        private readonly TextBox _text = new TextBox();
        public string CodeText => _text.Text;
        public PasteForm()
        {
            Text = "Вставить код из сканера";
            Width = 760; Height = 260; StartPosition = FormStartPosition.CenterParent;
            _text.Multiline = true; _text.Dock = DockStyle.Fill; _text.Font = new Font("Consolas", 10F); _text.ScrollBars = ScrollBars.Both;
            var ok = new Button { Text = "Добавить", Dock = DockStyle.Bottom, Height = 38, DialogResult = DialogResult.OK };
            Controls.Add(_text); Controls.Add(ok); AcceptButton = ok;
        }
    }
}