import { useCallback, useEffect, useRef, useState, type FormEvent, type MouseEvent } from 'react';
import {
  CaretDownIcon,
  ChatsIcon,
  PlusIcon,
  RobotIcon as Bot,
  PaperPlaneRightIcon as Send,
  SparkleIcon as Sparkles,
  TrashIcon,
  XIcon,
} from '@phosphor-icons/react';
import ReactMarkdown from 'react-markdown';
import { useAppDispatch, useAppSelector } from '@/app/hooks';
import {
  AGENT_MODEL_OPTIONS,
  appendAgentEvent,
  appendAgentMessagePart,
  appendAgentTextDelta,
  clearAgentChat,
  createAgentRun,
  deleteAgentConversation,
  fetchAgentConversationMessages,
  listAgentConversations,
  selectAgentChatHistory,
  selectAgentCreateRunLoading,
  selectAgentMessages,
  selectCurrentAgentConversationId,
  selectAgentConversation,
  selectSelectedAgentModelKey,
  setSelectedAgentModel,
} from '@/features/agent/store';
import { selectSelectedBudget } from '@/features/budget';
import { AGENT_CHAT_WEBSOCKET_EVENT, type WebSocketMessage } from '@/features/websocket/events';
import type { AgentChatHistoryItem, AgentChatMessage, AgentEventMessageData, MessagePart } from '@/features/agent';
import { LoadingState } from '@/utils';
import styles from './AgentChat.module.css';

const SUGGESTIONS = ['Summarize my spending', 'Find unusual transactions', 'Help plan next month'];
const TEXT_DELTA_EVENT = 'agent::chat::text_delta';
const AGENT_CHAT_STREAM_EVENT = 'pennywise::agent::chat::stream';
const AGENT_LOADING_EVENT = 'agent::chat::loading';
const TEXT_DELTA_CHARS_PER_SECOND = 90;
const TEXT_DELTA_MAX_FRAME_CHARS = 8;
const AUTO_SCROLL_BOTTOM_THRESHOLD_PX = 48;

function parseAgentStreamData(data: unknown) {
  if (typeof data === 'string') {
    try {
      const parsed = JSON.parse(data) as unknown;
      if (typeof parsed === 'string') {
        return parseAgentStreamData(parsed);
      }
      return parsed && typeof parsed === 'object' ? parsed as AgentEventMessageData : null;
    } catch {
      return null;
    }
  }

  if (data && typeof data === 'object') {
    return data as AgentEventMessageData;
  }

  return null;
}

function formatAgentEventData(data: unknown): string {
  const parsed = parseAgentStreamData(data);
  if (!parsed) {
    return typeof data === 'string' ? data : '';
  }

  const payload = parsed as Record<string, unknown>;
  const value = payload.message ?? payload.text ?? payload.delta ?? payload.content;
  if (typeof value === 'string') {
    return value;
  }

  return JSON.stringify(parsed);
}

function agentStreamMessageId(data: AgentEventMessageData | null) {
  if (!data) {
    return undefined;
  }

  const fallbackMessageId = (data as { messageId?: unknown }).messageId;
  if (typeof data.id === 'string' && data.id) {
    return data.id;
  }
  if (typeof fallbackMessageId === 'string' && fallbackMessageId) {
    return fallbackMessageId;
  }
  return undefined;
}

// function agentStreamText(data: unknown) {
//   if (typeof data === 'string') {
//     return data;
//   }
//   if (!data || typeof data !== 'object') {
//     return '';
//   }
//
//   const payload = data as Record<string, unknown>;
//   const value = payload.message ?? payload.text ?? payload.delta ?? payload.content;
//   return typeof value === 'string' ? value : '';
// }

function agentMessageLabel(eventName: string) {
  if (
    eventName === TEXT_DELTA_EVENT ||
    eventName === AGENT_CHAT_STREAM_EVENT ||
    eventName === 'agent::chat::message' ||
    eventName === AGENT_LOADING_EVENT ||
    eventName === 'agent::chat::tool_call' ||
    eventName === 'agent::chat::tool_call_start'
  ) {
    return 'Penny';
  }

  return eventName.replace(/^agent::chat::/, '');
}

function MarkdownAgentText({ text }: { text: string }) {
  return (
    <div className={styles.markdownMessage}>
      <ReactMarkdown skipHtml>{text}</ReactMarkdown>
    </div>
  );
}

function messagePartText(part: MessagePart) {
  if (typeof part.content === 'string') {
    return part.content;
  }
  if (typeof part.result === 'string') {
    return part.result;
  }
  return '';
}

function modelKeyFromMetadataModel(metadataModel: unknown) {
  if (typeof metadataModel !== 'string') {
    return undefined;
  }

  const [provider, modelName] = metadataModel.split('/');
  if (!provider || !modelName) {
    return undefined;
  }

  return AGENT_MODEL_OPTIONS.find((option) => option.provider === provider && option.modelName === modelName)?.key;
}

function AgentToolPart({ part }: { part: MessagePart }) {
  const displayName = part.displayName?.trim() || part.name?.trim() || 'Used tool';
  const summary = part.summary?.trim();
  const isPending = !summary;

  return (
    <div className={`${styles.toolPart} ${isPending ? styles.toolPartPending : ''}`}>
      <Sparkles size={14} weight="fill" className={styles.toolIcon} aria-hidden="true" />
      <div className={styles.toolText}>
        <span className={styles.toolName}>{displayName}</span>
        {summary && <span className={styles.toolSummary}>{summary}</span>}
      </div>
    </div>
  );
}

function streamToolMessage(value: unknown) {
  return value && typeof value === 'object' ? value as Record<string, unknown> : {};
}

function toolCallStartPart(message: unknown): MessagePart {
  const toolMessage = streamToolMessage(message);
  return {
    type: 'TOOL_CALL',
    id: typeof toolMessage.id === 'string' ? toolMessage.id : undefined,
    displayName: typeof toolMessage.displayName === 'string' ? toolMessage.displayName : 'Using tool',
  };
}

function AgentMessageBody({ message }: { message: AgentChatMessage }) {
  if (message.eventName === AGENT_LOADING_EVENT) {
    return (
      <div className={styles.loadingMessage}>
        <span className={styles.loadingSpinner} aria-hidden />
        <span className={styles.loadingText}>Thinking</span>
      </div>
    );
  }

  if (!message.parts?.length) {
    return <MarkdownAgentText text={message.text} />;
  }

  const hasTextPart = message.parts.some((part) => part.type.toUpperCase() === 'TEXT');

  return (
    <div className={styles.messageParts}>
      {message.text.trim() && !hasTextPart && <MarkdownAgentText text={message.text} />}
      {message.parts.map((part, index) => {
        const partType = part.type.toUpperCase();
        const key = part.id ?? `${message.id}-${index}`;

        if (partType === 'TOOL_CALL') {
          return <AgentToolPart key={key} part={part} />;
        }

        if (partType === 'TEXT') {
          const text = messagePartText(part);
          return text ? <MarkdownAgentText key={key} text={text} /> : null;
        }

        return null;
      })}
    </div>
  );
}

export function AgentChat() {
  const dispatch = useAppDispatch();
  const agentMessages = useAppSelector(selectAgentMessages);
  const chatHistory = useAppSelector(selectAgentChatHistory);
  const createRunLoading = useAppSelector(selectAgentCreateRunLoading);
  const currentConversationId = useAppSelector(selectCurrentAgentConversationId);
  const selectedModelKey = useAppSelector(selectSelectedAgentModelKey);
  const selectedBudget = useAppSelector(selectSelectedBudget);
  const [isOpen, setIsOpen] = useState(false);
  const [isHistoryOpen, setIsHistoryOpen] = useState(false);
  const [isModelOpen, setIsModelOpen] = useState(false);
  const [composerValue, setComposerValue] = useState('');
  const [isAwaitingAgentResponse, setIsAwaitingAgentResponse] = useState(false);
  const [conversationToDelete, setConversationToDelete] = useState<AgentChatHistoryItem | null>(null);
  const [isDeletingConversation, setIsDeletingConversation] = useState(false);
  const messagesRef = useRef<HTMLDivElement | null>(null);
  const composerInputRef = useRef<HTMLInputElement | null>(null);
  const historyMenuRef = useRef<HTMLDivElement | null>(null);
  const modelMenuRef = useRef<HTMLDivElement | null>(null);
  const shouldRefocusComposerRef = useRef(false);
  const pendingTextDeltaRef = useRef('');
  const pendingTextDeltaMessageIdRef = useRef<string | undefined>(undefined);
  const metadataSyncedConversationRef = useRef<string | null>(null);
  const textDeltaAnimationFrameRef = useRef<number | null>(null);
  const textDeltaFrameTimeRef = useRef<number | null>(null);
  const textDeltaCharBudgetRef = useRef(0);
  const shouldAutoScrollRef = useRef(true);
  const isSending = createRunLoading === LoadingState.PENDING;
  const hasSelectedBudget = Boolean(selectedBudget?.id);
  const lastMessage = agentMessages[agentMessages.length - 1];
  const shouldShowResponseLoader = isAwaitingAgentResponse && lastMessage?.role === 'user';
  const displayedAgentMessages = shouldShowResponseLoader
    ? [
        ...agentMessages,
        {
          id: 'agent-response-loader',
          role: 'assistant' as const,
          eventName: AGENT_LOADING_EVENT,
          text: '',
        },
      ]
    : agentMessages;
  const selectedConversation = currentConversationId
    ? chatHistory.find((conversation) => conversation.id === currentConversationId)
    : undefined;
  const selectedModel = AGENT_MODEL_OPTIONS.find((option) => option.key === selectedModelKey) ?? AGENT_MODEL_OPTIONS[0];
  const headerTitle = selectedConversation?.title ?? 'Penny Agent';
  const canOpenHistory = !isSending && chatHistory.length > 0;
  const canOpenModels = !isSending && hasSelectedBudget;
  const deleteConversationTitle = conversationToDelete?.title?.trim() || 'this chat';

  const focusComposerInput = useCallback(() => {
    if (!isOpen || !hasSelectedBudget) {
      return;
    }

    window.requestAnimationFrame(() => {
      composerInputRef.current?.focus();
    });
  }, [hasSelectedBudget, isOpen]);

  const scrollMessagesToBottom = useCallback(() => {
    const messages = messagesRef.current;
    if (!messages) {
      return;
    }

    messages.scrollTop = messages.scrollHeight;
  }, []);

  const handleMessagesScroll = useCallback(() => {
    const messages = messagesRef.current;
    if (!messages) {
      return;
    }

    const distanceFromBottom = messages.scrollHeight - messages.scrollTop - messages.clientHeight;
    shouldAutoScrollRef.current = distanceFromBottom <= AUTO_SCROLL_BOTTOM_THRESHOLD_PX;
  }, []);

  const appendTextDelta = useCallback(
    (text: string, messageId?: string) => {
      if (!text) {
        return;
      }

      dispatch(appendAgentTextDelta({ text, messageId }));
    },
    [dispatch],
  );

  const animateTextDelta = useCallback(
    (timestamp: number) => {
      const previousTimestamp = textDeltaFrameTimeRef.current ?? timestamp;
      const elapsedMs = Math.min(timestamp - previousTimestamp, 100);
      textDeltaFrameTimeRef.current = timestamp;
      textDeltaCharBudgetRef.current += (elapsedMs / 1000) * TEXT_DELTA_CHARS_PER_SECOND;

      const pendingText = pendingTextDeltaRef.current;
      if (pendingText && textDeltaCharBudgetRef.current >= 1) {
        const charCount = Math.min(
          pendingText.length,
          Math.floor(textDeltaCharBudgetRef.current),
          TEXT_DELTA_MAX_FRAME_CHARS,
        );

        pendingTextDeltaRef.current = pendingText.slice(charCount);
        textDeltaCharBudgetRef.current -= charCount;
        appendTextDelta(pendingText.slice(0, charCount), pendingTextDeltaMessageIdRef.current);
      }

      if (pendingTextDeltaRef.current) {
        textDeltaAnimationFrameRef.current = window.requestAnimationFrame(animateTextDelta);
        return;
      }

      textDeltaAnimationFrameRef.current = null;
      textDeltaFrameTimeRef.current = null;
      textDeltaCharBudgetRef.current = 0;
    },
    [appendTextDelta],
  );

  const scheduleTextDeltaAnimation = useCallback(() => {
    if (textDeltaAnimationFrameRef.current !== null) {
      return;
    }

    textDeltaAnimationFrameRef.current = window.requestAnimationFrame(animateTextDelta);
  }, [animateTextDelta]);

  const flushPendingTextDelta = useCallback(() => {
    if (textDeltaAnimationFrameRef.current !== null) {
      window.cancelAnimationFrame(textDeltaAnimationFrameRef.current);
      textDeltaAnimationFrameRef.current = null;
    }

    const text = pendingTextDeltaRef.current;
    pendingTextDeltaRef.current = '';
    textDeltaFrameTimeRef.current = null;
    textDeltaCharBudgetRef.current = 0;
    appendTextDelta(text, pendingTextDeltaMessageIdRef.current);
    pendingTextDeltaMessageIdRef.current = undefined;
  }, [appendTextDelta]);

  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        if (conversationToDelete && !isDeletingConversation) {
          setConversationToDelete(null);
          return;
        }
        if (isHistoryOpen) {
          setIsHistoryOpen(false);
          return;
        }
        if (isModelOpen) {
          setIsModelOpen(false);
          return;
        }
        setIsAwaitingAgentResponse(false);
        setIsOpen(false);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [conversationToDelete, isDeletingConversation, isHistoryOpen, isModelOpen, isOpen]);

  useEffect(() => {
    if (!isHistoryOpen) return;

    const handlePointerDown = (event: PointerEvent) => {
      if (!historyMenuRef.current?.contains(event.target as Node)) {
        setIsHistoryOpen(false);
      }
    };

    window.addEventListener('pointerdown', handlePointerDown);
    return () => window.removeEventListener('pointerdown', handlePointerDown);
  }, [isHistoryOpen]);

  useEffect(() => {
    if (!isModelOpen) return;

    const handlePointerDown = (event: PointerEvent) => {
      if (!modelMenuRef.current?.contains(event.target as Node)) {
        setIsModelOpen(false);
      }
    };

    window.addEventListener('pointerdown', handlePointerDown);
    return () => window.removeEventListener('pointerdown', handlePointerDown);
  }, [isModelOpen]);

  useEffect(() => {
    const handleAgentEvent = (event: Event) => {
      const message = (event as CustomEvent<WebSocketMessage>).detail;
      const parsedMsgData = parseAgentStreamData(message.data);
      const streamMessageId = agentStreamMessageId(parsedMsgData);

      if (message.eventName === AGENT_CHAT_STREAM_EVENT && parsedMsgData?.type === 'text_delta') {
        const text = typeof parsedMsgData.message === 'string' ? parsedMsgData.message : '';
        if (
          pendingTextDeltaRef.current &&
          pendingTextDeltaMessageIdRef.current &&
          streamMessageId &&
          pendingTextDeltaMessageIdRef.current !== streamMessageId
        ) {
          flushPendingTextDelta();
        }
        pendingTextDeltaMessageIdRef.current = streamMessageId;
        pendingTextDeltaRef.current += text;
        scheduleTextDeltaAnimation();
        return;
      }

      if (message.eventName === AGENT_CHAT_STREAM_EVENT && parsedMsgData?.type === 'tool_call_start') {
        flushPendingTextDelta();
        dispatch(appendAgentMessagePart({
          messageId: streamMessageId,
          part: toolCallStartPart(parsedMsgData.message),
        }));
        return;
      }

      if (
        message.eventName === AGENT_CHAT_STREAM_EVENT &&
        parsedMsgData?.type === 'tool_call' &&
        parsedMsgData.message &&
        typeof parsedMsgData.message === 'object'
      ) {
        flushPendingTextDelta();
        const toolMessage = streamToolMessage(parsedMsgData.message);
        dispatch(appendAgentMessagePart({
          messageId: streamMessageId,
          part: {
            type: 'TOOL_CALL',
            id: typeof toolMessage.id === 'string' ? toolMessage.id : undefined,
            displayName: typeof toolMessage.displayName === 'string' ? toolMessage.displayName : undefined,
            summary: typeof toolMessage.summary === 'string' ? toolMessage.summary : undefined,
            result: toolMessage.result,
          },
        }));
        return;
      }

      const text = formatAgentEventData(message.data);
      flushPendingTextDelta();
      dispatch(appendAgentEvent({ eventName: message.eventName, text }));
    };

    window.addEventListener(AGENT_CHAT_WEBSOCKET_EVENT, handleAgentEvent);
    return () => {
      window.removeEventListener(AGENT_CHAT_WEBSOCKET_EVENT, handleAgentEvent);
      if (textDeltaAnimationFrameRef.current !== null) {
        window.cancelAnimationFrame(textDeltaAnimationFrameRef.current);
        textDeltaAnimationFrameRef.current = null;
      }
      pendingTextDeltaRef.current = '';
      pendingTextDeltaMessageIdRef.current = undefined;
      textDeltaFrameTimeRef.current = null;
      textDeltaCharBudgetRef.current = 0;
    };
  }, [dispatch, flushPendingTextDelta, scheduleTextDeltaAnimation]);

  useEffect(() => {
    if (!isOpen) return;

    shouldAutoScrollRef.current = true;
    window.requestAnimationFrame(scrollMessagesToBottom);
    focusComposerInput();
  }, [focusComposerInput, isOpen, scrollMessagesToBottom]);

  useEffect(() => {
    if (!shouldRefocusComposerRef.current || isSending) {
      return;
    }

    shouldRefocusComposerRef.current = false;
    focusComposerInput();
  }, [focusComposerInput, isSending]);

  useEffect(() => {
    if (!isOpen || !selectedBudget?.id) return;

    dispatch(listAgentConversations());
  }, [dispatch, isOpen, selectedBudget?.id]);

  useEffect(() => {
    if (!currentConversationId) {
      metadataSyncedConversationRef.current = null;
      return;
    }

    if (metadataSyncedConversationRef.current === currentConversationId) {
      return;
    }

    const metadataModel = selectedConversation?.metadata?.model ?? selectedConversation?.metadata?.defaultModel;
    const modelKey = modelKeyFromMetadataModel(metadataModel);
    metadataSyncedConversationRef.current = currentConversationId;

    if (modelKey && modelKey !== selectedModelKey) {
      dispatch(setSelectedAgentModel(modelKey));
    }
  }, [
    currentConversationId,
    dispatch,
    selectedConversation?.metadata?.defaultModel,
    selectedConversation?.metadata?.model,
    selectedModelKey,
  ]);

  useEffect(() => {
    if (!isOpen || !shouldAutoScrollRef.current) return;

    scrollMessagesToBottom();
  }, [agentMessages, isOpen, scrollMessagesToBottom, shouldShowResponseLoader]);

  useEffect(() => {
    if (!isAwaitingAgentResponse) {
      return;
    }

    if (createRunLoading === LoadingState.ERROR) {
      setIsAwaitingAgentResponse(false);
      return;
    }

    if (
      lastMessage?.role === 'assistant' &&
      (lastMessage.text.trim() || (lastMessage.parts?.length ?? 0) > 0 || lastMessage.eventName === 'agent::chat::error')
    ) {
      setIsAwaitingAgentResponse(false);
    }
  }, [createRunLoading, isAwaitingAgentResponse, lastMessage]);

  const submitMessage = useCallback(
    async (message: string) => {
      const trimmedMessage = message.trim();
      if (!trimmedMessage || isSending || !hasSelectedBudget) {
        return;
      }

      setComposerValue('');
      shouldRefocusComposerRef.current = true;
      focusComposerInput();
      setIsHistoryOpen(false);
      setIsAwaitingAgentResponse(true);
      const run = await dispatch(
        createAgentRun({
          message: trimmedMessage,
          conversationId: currentConversationId ?? undefined,
        }),
      ).unwrap();
      dispatch(selectAgentConversation(run.conversationId ?? ''));
    },
    [currentConversationId, dispatch, focusComposerInput, hasSelectedBudget, isSending],
  );

  const handleSubmit = useCallback(
    (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      submitMessage(composerValue);
    },
    [composerValue, submitMessage],
  );

  const handleNewChat = useCallback(() => {
    if (isSending) {
      return;
    }

    flushPendingTextDelta();
    setIsHistoryOpen(false);
    setIsModelOpen(false);
    setComposerValue('');
    setIsAwaitingAgentResponse(false);
    dispatch(clearAgentChat());
  }, [dispatch, flushPendingTextDelta, isSending]);

  const handleSelectChat = useCallback(
    (conversationId: string) => {
      if (isSending) {
        return;
      }

      flushPendingTextDelta();
      setIsHistoryOpen(false);
      setIsModelOpen(false);
      setComposerValue('');
      setIsAwaitingAgentResponse(false);
      if (!conversationId) {
        dispatch(clearAgentChat());
        return;
      }

      dispatch(selectAgentConversation(conversationId));
      dispatch(fetchAgentConversationMessages(conversationId));
    },
    [dispatch, flushPendingTextDelta, isSending],
  );

  const handleRequestDeleteConversation = useCallback(
    (event: MouseEvent<HTMLButtonElement>, conversation: AgentChatHistoryItem) => {
      event.stopPropagation();
      if (isSending || isDeletingConversation) {
        return;
      }

      setConversationToDelete(conversation);
    },
    [isDeletingConversation, isSending],
  );

  const handleCancelDeleteConversation = useCallback(() => {
    if (isDeletingConversation) {
      return;
    }

    setConversationToDelete(null);
  }, [isDeletingConversation]);

  const handleConfirmDeleteConversation = useCallback(async () => {
    if (!conversationToDelete || isDeletingConversation) {
      return;
    }

    setIsDeletingConversation(true);
    try {
      flushPendingTextDelta();
      await dispatch(deleteAgentConversation(conversationToDelete.id)).unwrap();
      setConversationToDelete(null);
      setIsHistoryOpen(false);
      setIsAwaitingAgentResponse(false);
    } finally {
      setIsDeletingConversation(false);
    }
  }, [conversationToDelete, dispatch, flushPendingTextDelta, isDeletingConversation]);

  const handleSelectModel = useCallback(
    (modelKey: string) => {
      if (isSending || !hasSelectedBudget) {
        return;
      }

      dispatch(setSelectedAgentModel(modelKey));
      setIsModelOpen(false);
    },
    [dispatch, hasSelectedBudget, isSending],
  );

  return (
    <aside className={`${styles.agentPanel} ${isOpen ? styles.agentPanelOpen : ''}`} aria-label="Penny Agent chat">
      {!isOpen && (
        <button
          type="button"
          className={styles.launcher}
          aria-expanded={isOpen}
          aria-controls="agent-chat-panel"
          onClick={() => setIsOpen(true)}>
          <Sparkles size={18} />
          <span>Ask Penny</span>
        </button>
      )}

      <section
        id="agent-chat-panel"
        className={`${styles.panelCard} ${isOpen ? styles.panelCardOpen : ''}`}
        aria-hidden={!isOpen}>
        <header className={styles.header}>
          <div className={styles.agentMark}>
            <Bot size={20} />
          </div>
          <div className={styles.headerText}>
            <h2 id="agent-chat-title">{headerTitle}</h2>
          </div>
          <div className={styles.headerActions}>
            <div ref={historyMenuRef} className={styles.historyMenu}>
              <button
                type="button"
                className={styles.historyButton}
                aria-label="Open chat history"
                aria-haspopup="menu"
                aria-expanded={isHistoryOpen}
                title="Chat history"
                disabled={!canOpenHistory}
                onClick={() => setIsHistoryOpen((open) => !open)}>
                <ChatsIcon size={17} aria-hidden />
              </button>
              {isHistoryOpen && (
                <div className={styles.historyPopover} role="menu" aria-label="Chat history">
                  {chatHistory.map((conversation) => (
                    <div
                      key={conversation.id}
                      className={`${styles.historyRow} ${conversation.id === currentConversationId ? styles.historyOptionActive : ''
                        }`}
                    >
                      <button
                        type="button"
                        role="menuitem"
                        className={styles.historyOption}
                        onClick={() => handleSelectChat(conversation.id)}>
                        {conversation.title}
                      </button>
                      <button
                        type="button"
                        className={styles.historyDeleteButton}
                        aria-label={`Delete ${conversation.title}`}
                        disabled={isDeletingConversation}
                        onClick={(event) => handleRequestDeleteConversation(event, conversation)}>
                        <TrashIcon size={15} aria-hidden />
                      </button>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <button
              type="button"
              className={styles.newChatButton}
              aria-label="Start new chat"
              title="Start new chat"
              disabled={isSending}
              onClick={handleNewChat}>
              <PlusIcon size={17} />
            </button>
            <button
              type="button"
              className={styles.closeButton}
              aria-label="Close agent chat"
              onClick={() => {
                setIsAwaitingAgentResponse(false);
                setIsOpen(false);
              }}>
              <XIcon size={18} />
            </button>
          </div>
        </header>

        <div ref={messagesRef} className={styles.messages} onScroll={handleMessagesScroll}>
          {displayedAgentMessages.length === 0 && <div className={styles.emptyState}>ask questions about your budget</div>}
          {displayedAgentMessages.map((message) => (
            <div key={message.id} className={message.role === 'user' ? styles.userMessage : styles.agentMessage}>
              {message.role === 'assistant' && (
                <span className={styles.messageLabel}>
                  {agentMessageLabel(message.eventName ?? 'agent::chat::message')}
                </span>
              )}
              {message.role === 'assistant' ? <AgentMessageBody message={message} /> : <p>{message.text}</p>}
            </div>
          ))}
        </div>

        <div className={styles.suggestions} aria-label="Suggested prompts">
          {SUGGESTIONS.map((suggestion) => (
            <button
              key={suggestion}
              type="button"
              className={styles.suggestionButton}
              disabled={isSending || !hasSelectedBudget}
              onClick={() => submitMessage(suggestion)}>
              {suggestion}
            </button>
          ))}
        </div>

        <form className={styles.composer} onSubmit={handleSubmit}>
          <input
            ref={composerInputRef}
            type="text"
            placeholder={hasSelectedBudget ? 'How can I help you today?' : 'Select a budget to chat'}
            aria-label="Message Penny Agent"
            value={composerValue}
            disabled={isSending || !hasSelectedBudget}
            onChange={(event) => setComposerValue(event.target.value)}
          />
          <div className={styles.composerFooter}>
            <div ref={modelMenuRef} className={styles.modelMenu}>
              <button
                type="button"
                className={styles.modelButton}
                aria-haspopup="menu"
                aria-expanded={isModelOpen}
                title={selectedModel.label}
                disabled={!canOpenModels}
                onClick={() => setIsModelOpen((open) => !open)}>
                <span>{selectedModel.shortLabel ?? selectedModel.label}</span>
                <CaretDownIcon size={14} aria-hidden />
              </button>
              {isModelOpen && (
                <div className={styles.modelPopover} role="menu" aria-label="Agent model">
                  {AGENT_MODEL_OPTIONS.map((modelOption) => (
                    <button
                      key={modelOption.key}
                      type="button"
                      role="menuitem"
                      className={`${styles.modelOption} ${modelOption.key === selectedModelKey ? styles.modelOptionActive : ''
                        }`}
                      onClick={() => handleSelectModel(modelOption.key)}>
                      {modelOption.label}
                    </button>
                  ))}
                </div>
              )}
            </div>
            <button
              type="submit"
              className={styles.sendButton}
              aria-label="Send message"
              disabled={isSending || !hasSelectedBudget || !composerValue.trim()}>
              <Send size={17} />
            </button>
          </div>
        </form>
      </section>
      {conversationToDelete && (
        <div className={styles.confirmOverlay} role="presentation">
          <div
            className={styles.confirmDialog}
            role="dialog"
            aria-modal="true"
            aria-labelledby="delete-agent-chat-title">
            <h3 id="delete-agent-chat-title">Delete chat?</h3>
            <p>This will remove "{deleteConversationTitle}" from your chat history.</p>
            <div className={styles.confirmActions}>
              <button
                type="button"
                className={styles.confirmCancelButton}
                disabled={isDeletingConversation}
                onClick={handleCancelDeleteConversation}>
                Cancel
              </button>
              <button
                type="button"
                className={styles.confirmDeleteButton}
                disabled={isDeletingConversation}
                onClick={handleConfirmDeleteConversation}>
                {isDeletingConversation ? 'Deleting...' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </aside>
  );
}
