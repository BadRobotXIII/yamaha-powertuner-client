"""Combine our AAPT2 manifest/resources with our Fyne-built code and assets."""
import zipfile

with zipfile.ZipFile("CCU_Read_Only_Fyne.apk") as source:
    with zipfile.ZipFile("build/repacked.apk", "a") as target:
        for entry in source.infolist():
            if entry.filename == "classes.dex" or entry.filename.startswith(("lib/", "assets/")):
                target.writestr(entry.filename, source.read(entry), compress_type=zipfile.ZIP_DEFLATED)
