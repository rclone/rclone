# Go Mime Wrapper Library

Provides a parser for MIME messages

## Download/Install

Run `go get -u github.com/ProtonMail/go-mime`, or manually `git clone` the
repository into `$GOPATH/src/github.com/ProtonMail/go-mime`.


## Usage

The library can be used to extract the body and attachments from a MIME message

Example:
```go
printAccepter := gomime.NewMIMEPrinter()
bodyCollector := gomime.NewBodyCollector(printAccepter)
attachmentsCollector := gomime.NewAttachmentsCollector(bodyCollector)
mimeVisitor := gomime.NewMimeVisitor(attachmentsCollector)
err := gomime.VisitAll(bytes.NewReader(mmBodyData), h, mimeVisitor)
attachments := attachmentsCollector.GetAttachments(),
attachmentsHeaders :=	attachmentsCollector.GetAttHeaders()
bodyContent, bodyMimeType := bodyCollector.GetBody()
```
