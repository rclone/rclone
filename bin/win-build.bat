@echo off
echo Setting environment variables for mingw+WinFsp compile
set GOPATH=Z:\go
rem set PATH=C:\Program Files\mingw-w64\i686-7.1.0-win32-dwarf-rt_v5-rev0\mingw32\bin;%PATH%
set PATH=C:\Program Files\mingw-w64\x86_64-8.1.0-win32-seh-rt_v6-rev0\mingw64\bin;%GOPATH%/bin;%PATH%
set CPATH=C:\Program Files\WinFsp\inc\fuse;C:\Program Files (x86)\WinFsp\inc\fuse
