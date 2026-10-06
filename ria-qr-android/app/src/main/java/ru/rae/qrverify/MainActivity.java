package ru.rae.qrverify;
import android.app.Activity; import android.content.Intent; import android.net.Uri; import android.os.Bundle; import android.webkit.*; import android.widget.Toast;
import com.google.mlkit.vision.barcode.BarcodeScanner; import com.google.mlkit.vision.barcode.BarcodeScanning; import com.google.mlkit.vision.barcode.common.Barcode; import com.google.mlkit.vision.common.InputImage; import com.google.mlkit.vision.codescanner.*;
import org.json.JSONObject; import java.util.List;
public class MainActivity extends Activity {
 private static final int PICK_IMAGE=1001; private WebView webView; private GmsBarcodeScanner cameraScanner; private BarcodeScanner imageScanner;
 @Override protected void onCreate(Bundle b){super.onCreate(b); webView=new WebView(this); setContentView(webView); WebSettings s=webView.getSettings(); s.setJavaScriptEnabled(true); s.setDomStorageEnabled(true); s.setAllowFileAccess(true); s.setAllowContentAccess(true); webView.setWebChromeClient(new WebChromeClient()); webView.setWebViewClient(new WebViewClient()); webView.addJavascriptInterface(new NativeBridge(),"AndroidNative"); GmsBarcodeScannerOptions o=new GmsBarcodeScannerOptions.Builder().setBarcodeFormats(Barcode.FORMAT_QR_CODE).build(); cameraScanner=GmsBarcodeScanning.getClient(this,o); imageScanner=BarcodeScanning.getClient(); webView.loadUrl("file:///android_asset/index.html");}
 @Override protected void onActivityResult(int r,int c,Intent d){super.onActivityResult(r,c,d); if(r!=PICK_IMAGE||c!=RESULT_OK||d==null||d.getData()==null)return; try{InputImage i=InputImage.fromFilePath(this,d.getData()); imageScanner.process(i).addOnSuccessListener(this::onImageBarcodes).addOnFailureListener(e->sendError("Не удалось прочитать изображение: "+safe(e.getMessage())));}catch(Exception e){sendError("Не удалось открыть изображение: "+safe(e.getMessage()));}}
 private void onImageBarcodes(List<Barcode> bs){for(Barcode b:bs){String raw=b.getRawValue(); if(raw!=null&&!raw.isEmpty()){sendQr(raw);return;}} sendError("QR-код не найден на изображении");}
 private void sendQr(String raw){String js="window.onNativeQr("+JSONObject.quote(raw)+");"; webView.post(()->webView.evaluateJavascript(js,null));}
 private void sendError(String msg){String js="window.onNativeError("+JSONObject.quote(msg)+");"; webView.post(()->webView.evaluateJavascript(js,null));}
 private static String safe(String s){return s==null?"неизвестная ошибка":s;}
 public final class NativeBridge {
  @JavascriptInterface public void scanCamera(){runOnUiThread(()->cameraScanner.startScan().addOnSuccessListener(b->{String raw=b.getRawValue(); if(raw==null||raw.isEmpty())sendError("QR-код не содержит данных"); else sendQr(raw);}).addOnCanceledListener(()->{}).addOnFailureListener(e->sendError("Сканирование не удалось: "+safe(e.getMessage()))));}
  @JavascriptInterface public void chooseImage(){runOnUiThread(()->{Intent i=new Intent(Intent.ACTION_OPEN_DOCUMENT); i.addCategory(Intent.CATEGORY_OPENABLE); i.setType("image/*"); startActivityForResult(i,PICK_IMAGE);});}
  @JavascriptInterface public void openUrl(String url){runOnUiThread(()->{try{startActivity(new Intent(Intent.ACTION_VIEW,Uri.parse(url)));}catch(Exception e){Toast.makeText(MainActivity.this,"Не удалось открыть ссылку",Toast.LENGTH_SHORT).show();}});}
 }
 @Override protected void onDestroy(){try{imageScanner.close();}catch(Exception ignored){} if(webView!=null)webView.destroy(); super.onDestroy();}
}