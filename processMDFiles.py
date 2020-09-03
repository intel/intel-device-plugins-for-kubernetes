import os
import os.path
from pathlib import Path
from shutil import copyfile

fileExtension = ".md"
files = []
buildDir = "_build"


def getFiles():
    Path(buildDir).mkdir(parents=True, exist_ok=True)
    for dirpath, dirnames, filenames in os.walk("."):
        for filename in [f for f in filenames if f.endswith(fileExtension)]:
            if buildDir not in dirpath:
                lastDirName = os.path.split(dirpath)[1]
                if "." != lastDirName:
                    newFileName = lastDirName + "-" + filename
                else:
                    newFileName = filename
                filepath = os.path.join(dirpath,filename)
                print(filepath)
                newFilePath = os.path.join(buildDir,newFileName)
                #copyfile(filepath,newFilePath)
                files.append(newFileName)

getFiles()
