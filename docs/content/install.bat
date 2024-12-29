@echo off
setlocal enabledelayedexpansion


REM Check for admin rights
net session >nul 2>&1
if %errorLevel% equ 0 (
    echo Administrative privileges confirmed.
    echo --------------------------------------------------------
) else (
    echo Please run this script as an administrator.
    pause
    exit /b 1
)


REM Get windows versions
set "ps_script=$originalProtocol = [Net.ServicePointManager]::SecurityProtocol;$env:PSL1 = [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12;$response = Invoke-WebRequest -Uri 'https://api.github.com/repos/rclone/rclone/releases/latest' -UseBasicParsing -Method GET;$responseJson = $response | ConvertFrom-Json;foreach ($asset in $responseJson.assets) {if ($asset.name -match 'windows') {$asset.name | Out-File -FilePath 'rclone_win_versions.txt' -Append -Encoding ascii;$asset.browser_download_url | Out-File -FilePath 'rclone_win_download_links.txt' -Append -Encoding ascii;}};$env:PSL1 = [System.Net.ServicePointManager]::SecurityProtocol = $originalProtocol"
powershell -command "& {%ps_script%}"
set "versions=rclone_win_versions.txt"
set "download_links=rclone_win_download_links.txt"
set index1=0
set index2=0
for /f "delims=" %%A in (%versions%) do (
    set /a index1+=1
    echo !index1!. %%A
    set wins[!index1!]=%%A
)
for /f "delims=" %%B in (%download_links%) do (
    set /a index2+=1
    set downloads[!index2!]=%%B
)
del rclone_win_versions.txt
del rclone_win_download_links.txt


REM Get user input on windows version
:GET_INPUT
set /p "userInput=Select the appropriate option from above (between 1 and %index1% inclusive): "
:: Check if the input is a valid integer
for /f "tokens=1 delims=0123456789" %%a in ("%userInput%") do (
    echo Invalid input. Please try again.
    goto GET_INPUT
)
:: Check if the integer is within the specified range
if %userInput% lss 1 (
    echo Integer must be greater than or equal to 1.
    goto GET_INPUT
)
if %userInput% gtr %index1% (
    echo Integer must be less than or equal to %index1%.
    goto GET_INPUT
)
echo Selected !wins[%userInput%]!
echo --------------------------------------------------------


REM Download selected version
echo Downloading !downloads[%userInput%]!
set "download_script=$originalProtocol = [Net.ServicePointManager]::SecurityProtocol;$env:PSL1 = [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12;Invoke-WebRequest -Uri $link -UseBasicParsing -OutFile $name;$env:PSL1 = [System.Net.ServicePointManager]::SecurityProtocol = $originalProtocol"
powershell -Command "& {param($link, $name); %download_script%}" "!downloads[%userInput%]!" "!wins[%userInput%]!"
echo --------------------------------------------------------


REM Unzip downloaded zip
echo Unzipping !wins[%userInput%]!
powershell -Command "& {param($zip); Expand-Archive -Path $zip -Force}" "!wins[%userInput%]!"
echo --------------------------------------------------------


REM Create new directory for rclone
echo Creating base directory for rclone
set rclonePath=C:\rclone
mkdir %rclonePath%
echo --------------------------------------------------------


REM Copy rclone to created rclone path
echo Copy !wins[%userInput%]:~0,-4! executable to %rclonePath%
set unzip_path=.\!wins[%userInput%]:~0,-4!\!wins[%userInput%]:~0,-4!\rclone.exe
copy %unzip_path% %rclonePath%
echo --------------------------------------------------------


REM Add rclone directory to PATH
echo Adding %rclonePath% to PATH
if not defined PATH (
    setx PATH "%rclonePath%"
) else (
    setx PATH "%PATH%;%rclonePath%"
)
echo --------------------------------------------------------


echo Done
echo --------------------------------------------------------


endlocal