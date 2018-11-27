#coding:utf-8
import sys
sys.path.insert(0,"lib")
import evernote.edam.userstore.UserStore as UserStore
import evernote.edam.notestore.NoteStore as NoteStore
import evernote.edam.type.ttypes as Types
import evernote.edam.error.ttypes as Errors
from evernote.api.client import EvernoteClient
import sublime,sublime_plugin
import markdown2
import webbrowser

consumer_key = 'oparrish-4096'
consumer_secret ='c112c6417738f06a'
evernoteHost = "www.evernote.com"
callbackUrl = "http://127.0.0.1/sublimeevernote/callback"
settings = sublime.load_settings("SublimeEvernote.sublime-settings")


def get_evernote_client(token=None):
    if token:
        return EvernoteClient(token=token,service_host=evernoteHost, sandbox=False)
    else:
        return EvernoteClient(
            consumer_key=consumer_key,
            consumer_secret=consumer_secret,
            service_host=evernoteHost, 
            sandbox=False
        )    

class SendToEvernoteCommand(sublime_plugin.TextCommand):
    def __init__(self,view):
        self.view = view    
        self.window = sublime.active_window()

    def to_markdown_html(self):
        encoding = self.view.encoding()
        if encoding == 'Undefined':
            encoding = 'utf-8'
        elif encoding == 'Western (Windows 1252)':
            encoding = 'windows-1252'


        sels = self.view.sel()
        contents = ''
        if sels:
            for sel in sels: contents += self.view.substr(sel) + '\n\n'   

        if not contents.strip():
            region = sublime.Region(0L, self.view.size())
            contents = self.view.substr(region) 

        markdown_html = markdown2.markdown(contents, extras=['footnotes', 'fenced-code-blocks', 'cuddled-lists', 'code-friendly', 'metadata'])
        return markdown_html

    def connect(self,callback,**kwargs):
        sublime.status_message("authenticate..., please wait...")   
        client = get_evernote_client()    
        request_token = client.get_request_token(callbackUrl)

        def on_verifier(verifier):
            access_token =  client.get_access_token(request_token['oauth_token'],request_token['oauth_token_secret'],verifier)
            settings.set('access_token',access_token)
            sublime.save_settings('SublimeEvernote.sublime-settings') 
            sublime.status_message("authenticate ok")
            callback(**kwargs)

        webbrowser.open(client.get_authorize_url(request_token))
        self.window.show_input_panel("type Verifier (required):",'',on_verifier,None,None) 

    def send_note(self,**kwargs):
        access_token = settings.get('access_token')
        client,noteStore = None,None
        if access_token :
            client = get_evernote_client(token=access_token)    
        else:
            return self.connect(self.send_note,**kwargs)
            
        try:
            noteStore = client.get_note_store()     
        except Exception as e:
            if sublime.ok_cancel_dialog('error %s! retry?'%e):
                self.connect(self.send_note,**kwargs)

        markdown_html = self.to_markdown_html()

        def sendnote(title,tags):
            note = Types.Note()
            note.title = title.encode('utf-8')
            note.content = '<?xml version="1.0" encoding="UTF-8"?>'
            note.content += '<!DOCTYPE en-note SYSTEM "http://xml.evernote.com/pub/enml2.dtd">'
            note.content += '<en-note>%s'%markdown_html.encode('utf-8')
            note.content += '</en-note>'
            note.tagNames = tags and tags.split(",") or []
            try:
                sublime.status_message("please wait...")   
                cnote = noteStore.createNote(access_token, note)   
                sublime.status_message("send success guid:%s"%cnote.guid)  
                sublime.message_dialog("success") 
            except Errors.EDAMUserException as e:
                args = dict(title=title,tags=tags)
                if e.errorCode == 9:
                    self.connect(self.send_note,**args)
                else:
                    if sublime.ok_cancel_dialog('error %s! retry?'%e):
                        self.connect(self.send_note,**args)
            except  Exception as e:
                sublime.error_message('error %s'%e)

        def on_title(title):
            def on_tags(tags):
                sendnote(title,tags)
            if not 'tags' in markdown_html.metadata:
                self.window.show_input_panel("Tags (Optional)::","",on_tags,None,None) 
            else:
                sendnote(title, markdown_html.metadata['tags'])

        if not(kwargs.get("title") or 'title' in markdown_html.metadata):
            self.window.show_input_panel("Title (required)::","",on_title,None,None)
        elif not kwargs.get("tags"):
            on_title(markdown_html.metadata['title'])
        else:    
            sendnote(kwargs.get("title"),kwargs.get("tags")) 

    def run(self, edit):
        if not settings.get("access_token"):
            self.connect(self.send_note)
        else:
            self.send_note()
