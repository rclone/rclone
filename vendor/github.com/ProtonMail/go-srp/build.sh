#!/bin/bash

#  The MIT License
#
#  Copyright (c) 2019 Proton Technologies AG
#
#  Permission is hereby granted, free of charge, to any person obtaining a copy
#  of this software and associated documentation files (the "Software"), to deal
#  in the Software without restriction, including without limitation the rights
#  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
#  copies of the Software, and to permit persons to whom the Software is
#  furnished to do so, subject to the following conditions:
#
#  The above copyright notice and this permission notice shall be included in
#  all copies or substantial portions of the Software.
#
#  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
#  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
#  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
#  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
#  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
#  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
#  THE SOFTWARE.

SCRIPT_LOCATION=$(cd $(dirname $0);echo $PWD)

GO111MODULE=off

OUTPUT_PATH="dist"
ANDROID_OUT=${OUTPUT_PATH}/"Android"
IOS_OUT=${OUTPUT_PATH}/"iOS"
macOS_OUT=${OUTPUT_PATH}/"macOS"

printf "\e[0;32mStart Building iOS framework .. Location: ${IOS_OUT} \033[0m\n\n"

gomobile bind -target ios -ldflags="-s -w -X srp.Version=$(git describe --always --long --dirty)" -o ${IOS_OUT}/Srp.framework 

printf "\e[0;32mStart Building macOS framework .. Location: ${IOS_OUT} \033[0m\n\n"

# this part need the customized gomobile bind
gomobile bind -target macos -ldflags="-s -w -X srp.Version=$(git describe --always --long --dirty)" -o ${macOS_OUT}/Srp.framework 

#for vpn 
#gomobile bind -target ios -o ${IOS_OUT}/Srp_vpn.framework -tags openpgp -ldflags="-s -w"

printf "\e[0;32mStart Building Android lib .. Location: ${ANDROID_OUT} \033[0m\n\n"

gomobile bind -target android -ldflags="-s -w -X srp.Version=$(git describe --always --long --dirty)" -o ${ANDROID_OUT}/srp.aar

printf "\e[0;32mAll Done. \033[0m\n\n"


