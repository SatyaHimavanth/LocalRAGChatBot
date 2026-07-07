import React, { useState, useEffect, useRef } from "react";
import { Events } from "@wailsio/runtime";
import { SendMessage, IngestFile } from "../bindings/changeme/internal/app/chatservice";

// Standard pure SVG icon wrappers
const PlusIcon = () => <svg className="menu-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg>;
const SearchIcon = () => <svg className="menu-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>;
const LibraryIcon = () => <svg className="menu-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"></path><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"></path></svg>;
const SendIcon = () => <svg className="send-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="22" y1="2" x2="11" y2="13"></line><polygon points="22 2 15 22 11 13 2 9 22 2"></polygon></svg>;

interface Message {
  id: string;
  sender: "user" | "ai";
  text: string;
}

interface Chat {
  id: number;
  title: string;
  messages: Message[];
  created: Date;
}

interface Collection {
  id: number;
  name: string;
  docCount: number;
}

interface IngestedDocument {
  id: string;
  collectionId: number;
  filename: string;
  wordCount: number;
  timestamp: string;
}

export default function App() {
  const [activeTab, setActiveTab] = useState<"chat" | "search" | "collections">("chat");
  const [chats, setChats] = useState<Chat[]>([
    {
      id: 1,
      title: "First RAG Chat",
      created: new Date(),
      messages: [],
    }
  ]);
  const [activeChatId, setActiveChatId] = useState<number>(1);
  const [inputMessage, setInputMessage] = useState("");
  const [isGenerating, setIsGenerating] = useState(false);

  // Search Page State
  const [searchQuery, setSearchQuery] = useState("");
  
  // Collections Page State
  const [collections, setCollections] = useState<Collection[]>([
    { id: 1, name: "Knowledge Base", docCount: 0 }
  ]);
  const [activeCollectionId, setActiveChatCollectionId] = useState<number>(1);
  const [newCollName, setNewCollectionName] = useState("");
  const [fileName, setFileName] = useState("");
  const [fileContent, setFileContent] = useState("");
  const [ingestionStatus, setIngestionStatus] = useState<string>("");

  // Ingested Documents Registry
  const [ingestedDocs, setIngestedDocs] = useState<IngestedDocument[]>([]);

  // Ref tracking to guarantee streaming tokens always append to the correct targeted chat session
  const activeChatIdRef = useRef<number>(activeChatId);
  useEffect(() => {
    activeChatIdRef.current = activeChatId;
  }, [activeChatId]);

  const activeChat = chats.find((c) => c.id === activeChatId) || chats[0];

  // Token streaming handler
  useEffect(() => {
    const offToken = Events.On("chat:token", (e: any) => {
      const targetSessionId = e.data.sessionId;
      setChats((prevChats) =>
        prevChats.map((chat) => {
          if (chat.id === targetSessionId) {
            const lastMsg = chat.messages[chat.messages.length - 1];
            if (lastMsg && lastMsg.sender === "ai") {
              const updatedMessages = [...chat.messages];
              updatedMessages[updatedMessages.length - 1] = {
                ...lastMsg,
                text: lastMsg.text + e.data.token,
              };
              return { ...chat, messages: updatedMessages };
            } else {
              return {
                ...chat,
                messages: [
                  ...chat.messages,
                  { id: Math.random().toString(), sender: "ai", text: e.data.token }
                ]
              };
            }
          }
          return chat;
        })
      );
    });

    const offDone = Events.On("chat:done", () => {
      setIsGenerating(false);
    });

    return () => {
      offToken();
      offDone();
    };
  }, []);

  // Handle New Chat logic
  const handleNewChat = () => {
    // If there is already an empty chat, do not create a new one, just switch to it
    const emptyChat = chats.find((c) => c.messages.length === 0);
    if (emptyChat) {
      setActiveChatId(emptyChat.id);
      setActiveTab("chat");
      return;
    }

    const nextId = chats.length > 0 ? Math.max(...chats.map((c) => c.id)) + 1 : 1;
    const newChatObj: Chat = {
      id: nextId,
      title: "New Chat",
      created: new Date(),
      messages: [],
    };

    setChats([newChatObj, ...chats]);
    setActiveChatId(nextId);
    setActiveTab("chat");
  };

  // Handle Send Message
  const handleSend = async () => {
    if (!inputMessage.trim() || isGenerating) return;

    const userPrompt = inputMessage;
    const targetedSessionId = activeChatId; // Freeze session reference
    setInputMessage("");
    setIsGenerating(true);

    // 1. Rename the chat if it's currently named "New Chat"
    let updatedTitle = activeChat.title;
    if (activeChat.title === "New Chat" || activeChat.title === "First RAG Chat") {
      updatedTitle = userPrompt.length > 25 ? userPrompt.substring(0, 25) + "..." : userPrompt;
    }

    // 2. Insert User prompt
    const userMsg: Message = { id: Math.random().toString(), sender: "user", text: userPrompt };
    setChats((prevChats) =>
      prevChats.map((c) => {
        if (c.id === targetedSessionId) {
          return {
            ...c,
            title: updatedTitle,
            messages: [...c.messages, userMsg],
          };
        }
        return c;
      })
    );

    // 3. Trigger Wails Chat completion
    try {
      await SendMessage(targetedSessionId, activeCollectionId, userPrompt);
    } catch (err) {
      console.error(err);
      setIsGenerating(false);
    }
  };

  // Handle Collection Creation
  const handleCreateCollection = () => {
    if (!newCollName.trim()) return;
    const nextId = collections.length > 0 ? Math.max(...collections.map((col) => col.id)) + 1 : 1;
    setCollections([...collections, { id: nextId, name: newCollName, docCount: 0 }]);
    setNewCollectionName("");
  };

  // Handle File Ingest submission
  const handleIngestFile = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!fileContent.trim()) return;

    // 1. Clean, sanitize, and validate the filename
    let rawName = fileName.trim();
    if (!rawName) {
      rawName = "pasted-text-document";
    }
    // Sanitize non-alphanumeric characters but preserve extension
    let sanitizedName = rawName.replace(/[^a-zA-Z0-9.-]/g, "_");
    if (!sanitizedName.includes(".")) {
      sanitizedName += ".txt"; // Add default .txt fallback if missing extension
    }

    setIngestionStatus("In progress... Vectorizing and chunking contents locally.");

    try {
      // Call the strongly bound typed IngestFile function directly
      await IngestFile(activeCollectionId, sanitizedName, fileContent);
      
      // Update collections and add to document registry
      setCollections((prevCols) =>
        prevCols.map((col) =>
          col.id === activeCollectionId ? { ...col, docCount: col.docCount + 1 } : col
        )
      );

      const words = fileContent.split(/\s+/).length;
      const newDoc: IngestedDocument = {
        id: Math.random().toString(),
        collectionId: activeCollectionId,
        filename: sanitizedName,
        wordCount: words,
        timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
      };

      setIngestedDocs((prevDocs) => [newDoc, ...prevDocs]);
      setFileName("");
      setFileContent("");
      setIngestionStatus("Success! File split, embedded, and ingested successfully.");
      setTimeout(() => setIngestionStatus(""), 4000);
    } catch (err) {
      setIngestionStatus("");
      alert("Ingestion error: " + err);
    }
  };

  // Date Grouper for Sidebar chats
  const groupedChats = () => {
    const today: Chat[] = [];
    const yesterday: Chat[] = [];
    const older: Chat[] = [];

    const now = new Date();
    const oneDay = 24 * 60 * 60 * 1000;

    chats.forEach((chat) => {
      const diff = now.getTime() - chat.created.getTime();
      if (diff < oneDay) {
        today.push(chat);
      } else if (diff < 2 * oneDay) {
        yesterday.push(chat);
      } else {
        older.push(chat);
      }
    });

    return { today, yesterday, older };
  };

  const { today, yesterday, older } = groupedChats();

  return (
    <div className="app-container">
      {/* SIDEBAR PANEL */}
      <div className="sidebar">
        <div className="sidebar-header">
          <div className="sidebar-brand">
            <span className="brand-dot"></span>
            LocalRAG Chat
          </div>
        </div>

        <div className="sidebar-menu">
          <button className="menu-item" onClick={handleNewChat}>
            <PlusIcon /> New Chat
          </button>
          <button
            className={`menu-item ${activeTab === "search" ? "active" : ""}`}
            onClick={() => setActiveTab("search")}
          >
            <SearchIcon /> Universal Search
          </button>
          <button
            className={`menu-item ${activeTab === "collections" ? "active" : ""}`}
            onClick={() => setActiveTab("collections")}
          >
            <LibraryIcon /> Collections
          </button>
        </div>

        <div className="divider" />

        <div className="sidebar-chats-label">Chats History</div>
        <div className="sidebar-chats-container">
          {today.length > 0 && (
            <div className="chats-group">
              <div className="chats-group-title">Today</div>
              {today.map((chat) => (
                <button
                  key={chat.id}
                  className={`chat-list-item ${
                    activeTab === "chat" && activeChatId === chat.id ? "active" : ""
                  }`}
                  onClick={() => {
                    setActiveChatId(chat.id);
                    setActiveTab("chat");
                  }}
                >
                  {chat.title}
                </button>
              ))}
            </div>
          )}

          {yesterday.length > 0 && (
            <div className="chats-group">
              <div className="chats-group-title">Yesterday</div>
              {yesterday.map((chat) => (
                <button
                  key={chat.id}
                  className={`chat-list-item ${
                    activeTab === "chat" && activeChatId === chat.id ? "active" : ""
                  }`}
                  onClick={() => {
                    setActiveChatId(chat.id);
                    setActiveTab("chat");
                  }}
                >
                  {chat.title}
                </button>
              ))}
            </div>
          )}

          {older.length > 0 && (
            <div className="chats-group">
              <div className="chats-group-title">Older</div>
              {older.map((chat) => (
                <button
                  key={chat.id}
                  className={`chat-list-item ${
                    activeTab === "chat" && activeChatId === chat.id ? "active" : ""
                  }`}
                  onClick={() => {
                    setActiveChatId(chat.id);
                    setActiveTab("chat");
                  }}
                >
                  {chat.title}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* MAIN MAIN PANEL */}
      <div className="main-panel">
        {activeTab === "chat" && (
          <div className="chat-window">
            <div className="chat-header">
              <span className="chat-title">{activeChat.title}</span>
              <span className="collection-meta" style={{ fontSize: "13px" }}>
                Targeting Collection: {collections.find((c) => c.id === activeCollectionId)?.name}
              </span>
            </div>

            <div className="chat-messages">
              {activeChat.messages.length === 0 ? (
                <div className="welcome-screen">
                  <div className="welcome-logo" />
                  <h1 className="welcome-title">Ask Knowledge Base</h1>
                  <p className="welcome-subtitle">
                    Type a query or load file contexts inside collections to retrieve local, vectorized segments.
                  </p>
                </div>
              ) : (
                activeChat.messages.map((msg) => (
                  <div key={msg.id} className={`message-bubble-wrapper ${msg.sender}`}>
                    <span className="message-sender">{msg.sender === "user" ? "You" : "LocalRAG AI"}</span>
                    <div className="message-bubble">{msg.text}</div>
                  </div>
                ))
              )}
            </div>

            <div className="chat-input-area">
              <div className="input-container">
                <input
                  type="text"
                  className="chat-input"
                  placeholder="Ask any question..."
                  value={inputMessage}
                  onChange={(e) => setInputMessage(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleSend()}
                />
                <button className="send-btn" onClick={handleSend}>
                  <SendIcon />
                </button>
              </div>
            </div>
          </div>
        )}

        {activeTab === "search" && (
          <div className="page-container">
            <h1 className="page-title">Universal Retrieval Search</h1>
            <input
              type="text"
              className="search-box"
              placeholder="Search terms across FTS5 keyword index and Vector embeddings..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
            <div className="search-results">
              <div className="search-card">
                <h3 style={{ fontSize: "15px", marginBottom: "8px" }}>Search across Local Collections</h3>
                <p style={{ color: "var(--text-muted)", fontSize: "14px", lineHeight: "1.5" }}>
                  Perform keyword and vector queries dynamically by entering your prompt query inside the main Chat Window. The hybrid search system is fully wired into your primary message interface.
                </p>
              </div>
            </div>
          </div>
        )}

        {activeTab === "collections" && (
          <div className="page-container">
            <h1 className="page-title">Knowledge Collections</h1>

            <div className="collections-grid">
              {collections.map((col) => (
                <div
                  key={col.id}
                  className={`collection-card ${col.id === activeCollectionId ? "active" : ""}`}
                  onClick={() => setActiveChatCollectionId(col.id)}
                >
                  <div className="collection-name">{col.name}</div>
                  <div className="collection-meta">{col.docCount} Documents Ingested</div>
                </div>
              ))}

              <div className="collection-card add-collection-card">
                <input
                  type="text"
                  placeholder="New Collection Name..."
                  value={newCollName}
                  onChange={(e) => setNewCollectionName(e.target.value)}
                  style={{
                    background: "transparent",
                    border: "none",
                    outline: "none",
                    color: "#fff",
                    textAlign: "center",
                    fontSize: "13px",
                    marginBottom: "10px",
                    width: "100%",
                  }}
                />
                <button className="primary-btn" onClick={handleCreateCollection} style={{ padding: "6px 12px", fontSize: "12px" }}>
                  + Create
                </button>
              </div>
            </div>

            {/* Ingested Documents List view */}
            <div className="ingest-section" style={{ marginBottom: "30px" }}>
              <h2 style={{ fontSize: "18px", marginBottom: "12px" }}>Ingested Documents Registry</h2>
              {ingestedDocs.length === 0 ? (
                <p style={{ color: "var(--text-muted)", fontSize: "13.5px" }}>No documents ingested yet in this session.</p>
              ) : (
                <div style={{ display: "flex", flexDirection: "column", gap: "10px" }}>
                  {ingestedDocs.filter(d => d.collectionId === activeCollectionId).map((doc) => (
                    <div key={doc.id} style={{
                      display: "flex",
                      justifyContent: "space-between",
                      alignItems: "center",
                      background: "rgba(255, 255, 255, 0.03)",
                      padding: "10px 16px",
                      borderRadius: "8px",
                      border: "1px solid var(--panel-border)"
                    }}>
                      <span style={{ fontSize: "14px", fontWeight: "500" }}>{doc.filename}</span>
                      <span style={{ fontSize: "12px", color: "var(--text-muted)" }}>
                        {doc.wordCount} words • {doc.timestamp}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="ingest-section">
              <h2 style={{ fontSize: "18px", marginBottom: "6px" }}>Vectorize & Ingest Local Document</h2>
              <p style={{ color: "var(--text-muted)", fontSize: "13px" }}>
                Target Collection: <strong style={{ color: "#ff4d4d" }}>{collections.find((c) => c.id === activeCollectionId)?.name}</strong>
              </p>

              <form className="ingest-form" onSubmit={handleIngestFile}>
                <input
                  type="text"
                  className="ingest-input"
                  placeholder="Filename (e.g. document.txt or 'pasted-document')"
                  value={fileName}
                  onChange={(e) => setFileName(e.target.value)}
                />
                <textarea
                  className="ingest-input ingest-textarea"
                  placeholder="Paste text contents here to split, embed, and store dynamically..."
                  value={fileContent}
                  onChange={(e) => setFileContent(e.target.value)}
                  required
                />
                
                {ingestionStatus && (
                  <p style={{
                    fontSize: "13.5px",
                    color: ingestionStatus.includes("Success") ? "#4caf50" : "#ff9800",
                    fontWeight: "500",
                    margin: "5px 0"
                  }}>{ingestionStatus}</p>
                )}

                <button type="submit" className="primary-btn" style={{ alignSelf: "flex-start" }}>
                  Ingest & Vectorize Document
                </button>
              </form>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
