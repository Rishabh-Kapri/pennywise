import { useCallback, useEffect, useRef, useState } from 'react';
import {
  ActivityIndicator,
  FlatList,
  KeyboardAvoidingView,
  Modal,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  TextInput,
  View,
  type ListRenderItem
} from 'react-native';
import { Bot, ChevronDown, MessagesSquare, Plus, Send, Sparkles, Trash2, X } from 'lucide-react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useAppDispatch, useAppSelector } from '../../../app/hooks';
import { AppText } from '../../../components/AppText';
import { LoadingState } from '../../../utils/constants';
import { colors, radii, spacing } from '../../../theme';
import {
  AGENT_MODEL_OPTIONS,
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
  setSelectedAgentModel
} from '../store/agentSlice';
import type { AgentChatHistoryItem, AgentChatMessage, MessagePart } from '../types';
import { selectSelectedBudget } from '../../budget/store/budgetSlice';

const SUGGESTIONS = ['Summarize my spending', 'Find unusual transactions', 'Help plan next month'];
const TEXT_DELTA_EVENT = 'agent::chat::text_delta';
const AGENT_CHAT_STREAM_EVENT = 'pennywise::agent::chat::stream';
const AGENT_LOADING_EVENT = 'agent::chat::loading';

function agentMessageLabel(eventName: string) {
  if (
    eventName === TEXT_DELTA_EVENT ||
    eventName === AGENT_CHAT_STREAM_EVENT ||
    eventName === 'agent::chat::message' ||
    eventName === 'agent::chat::tool_call' ||
    eventName === 'agent::chat::tool_call_start'
  ) {
    return 'Penny';
  }

  return eventName.replace(/^agent::chat::/, '');
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

  return (
    <View style={[styles.toolPart, !summary && styles.toolPartPending]}>
      <Sparkles size={14} color={colors.primaryLight} />
      <View style={styles.toolText}>
        <AppText weight="semibold" style={styles.toolName}>
          {displayName}
        </AppText>
        {summary ? (
          <AppText muted style={styles.toolSummary}>
            {summary}
          </AppText>
        ) : (
          <AppText muted style={styles.toolSummary}>
            Working
          </AppText>
        )}
      </View>
    </View>
  );
}

function AgentMessageBody({ message }: { message: AgentChatMessage }) {
  if (message.eventName === AGENT_LOADING_EVENT) {
    return (
      <View style={styles.loadingMessage}>
        <ActivityIndicator size="small" color={colors.primaryLight} />
        <AppText muted style={styles.loadingText}>
          Thinking
        </AppText>
      </View>
    );
  }

  if (!message.parts?.length) {
    return <AppText style={styles.messageText}>{message.text}</AppText>;
  }

  const hasTextPart = message.parts.some((part) => part.type.toUpperCase() === 'TEXT');

  return (
    <View style={styles.messageParts}>
      {message.text.trim() && !hasTextPart ? <AppText style={styles.messageText}>{message.text}</AppText> : null}
      {message.parts.map((part, index) => {
        const partType = part.type.toUpperCase();
        const key = part.id ?? `${message.id}-${index}`;

        if (partType === 'TOOL_CALL') {
          return <AgentToolPart key={key} part={part} />;
        }

        if (partType === 'TEXT') {
          const text = messagePartText(part);
          return text ? (
            <AppText key={key} style={styles.messageText}>
              {text}
            </AppText>
          ) : null;
        }

        return null;
      })}
    </View>
  );
}

function ChatMessage({ message }: { message: AgentChatMessage }) {
  const isUser = message.role === 'user';

  return (
    <View style={[styles.messageBubble, isUser ? styles.userMessage : styles.agentMessage]}>
      {!isUser ? (
        <AppText weight="bold" style={styles.messageLabel}>
          {agentMessageLabel(message.eventName ?? 'agent::chat::message')}
        </AppText>
      ) : null}
      {isUser ? <AppText style={styles.userMessageText}>{message.text}</AppText> : <AgentMessageBody message={message} />}
    </View>
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
  const metadataSyncedConversationRef = useRef<string | null>(null);
  const messagesListRef = useRef<FlatList<AgentChatMessage>>(null);
  const composerInputRef = useRef<TextInput>(null);
  const shouldRefocusComposerRef = useRef(false);
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
          text: ''
        }
      ]
    : agentMessages;
  const selectedConversation = currentConversationId ? chatHistory.find((conversation) => conversation.id === currentConversationId) : undefined;
  const selectedModel = AGENT_MODEL_OPTIONS.find((option) => option.key === selectedModelKey) ?? AGENT_MODEL_OPTIONS[0];
  const headerTitle = selectedConversation?.title ?? 'Penny Agent';
  const canOpenHistory = !isSending && chatHistory.length > 0;
  const canOpenModels = !isSending && hasSelectedBudget;
  const deleteConversationTitle = conversationToDelete?.title?.trim() || 'this chat';

  const focusComposerInput = useCallback(() => {
    if (!isOpen || !hasSelectedBudget) {
      return;
    }

    requestAnimationFrame(() => {
      composerInputRef.current?.focus();
    });
  }, [hasSelectedBudget, isOpen]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    focusComposerInput();
  }, [focusComposerInput, isOpen]);

  useEffect(() => {
    if (!shouldRefocusComposerRef.current || isSending) {
      return;
    }

    shouldRefocusComposerRef.current = false;
    focusComposerInput();
  }, [focusComposerInput, isSending]);

  useEffect(() => {
    if (!isOpen || !selectedBudget?.id) {
      return;
    }

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
    selectedModelKey
  ]);

  useEffect(() => {
    if (!isOpen || displayedAgentMessages.length === 0) {
      return;
    }

    const timer = setTimeout(() => {
      messagesListRef.current?.scrollToEnd({ animated: true });
    }, 40);

    return () => clearTimeout(timer);
  }, [displayedAgentMessages.length, isOpen]);

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
    (message: string) => {
      const trimmedMessage = message.trim();
      if (!trimmedMessage || isSending || !hasSelectedBudget) {
        return;
      }

      setComposerValue('');
      shouldRefocusComposerRef.current = true;
      focusComposerInput();
      setIsHistoryOpen(false);
      setIsAwaitingAgentResponse(true);
      dispatch(
        createAgentRun({
          message: trimmedMessage,
          conversationId: currentConversationId ?? undefined
        })
      );
    },
    [currentConversationId, dispatch, focusComposerInput, hasSelectedBudget, isSending]
  );

  const handleNewChat = useCallback(() => {
    if (isSending) {
      return;
    }

    setIsHistoryOpen(false);
    setIsModelOpen(false);
    setComposerValue('');
    setIsAwaitingAgentResponse(false);
    dispatch(clearAgentChat());
  }, [dispatch, isSending]);

  const handleSelectChat = useCallback(
    (conversationId: string) => {
      if (isSending) {
        return;
      }

      setIsHistoryOpen(false);
      setIsModelOpen(false);
      setComposerValue('');
      setIsAwaitingAgentResponse(false);
      dispatch(selectAgentConversation(conversationId));
      dispatch(fetchAgentConversationMessages(conversationId));
    },
    [dispatch, isSending]
  );

  const handleRequestDeleteConversation = useCallback(
    (conversation: AgentChatHistoryItem) => {
      if (isSending || isDeletingConversation) {
        return;
      }

      setConversationToDelete(conversation);
    },
    [isDeletingConversation, isSending]
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
      await dispatch(deleteAgentConversation(conversationToDelete.id)).unwrap();
      setConversationToDelete(null);
      setIsHistoryOpen(false);
    } finally {
      setIsDeletingConversation(false);
    }
  }, [conversationToDelete, dispatch, isDeletingConversation]);

  const handleSelectModel = useCallback(
    (modelKey: string) => {
      if (isSending || !hasSelectedBudget) {
        return;
      }

      dispatch(setSelectedAgentModel(modelKey));
      setIsModelOpen(false);
    },
    [dispatch, hasSelectedBudget, isSending]
  );

  const closePanel = useCallback(() => {
    setIsHistoryOpen(false);
    setIsModelOpen(false);
    setIsAwaitingAgentResponse(false);
    setIsOpen(false);
  }, []);

  const renderMessage = useCallback<ListRenderItem<AgentChatMessage>>(({ item }) => <ChatMessage message={item} />, []);

  return (
    <>
      {!isOpen ? (
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Ask Penny"
          style={({ pressed }) => [styles.launcher, pressed && styles.pressed]}
          onPress={() => setIsOpen(true)}
        >
          <Sparkles size={18} color="#ffffff" />
          <AppText weight="bold" style={styles.launcherText}>
            Ask Penny
          </AppText>
        </Pressable>
      ) : null}

      <Modal visible={isOpen} animationType="slide" presentationStyle="fullScreen" onRequestClose={closePanel}>
        <SafeAreaView style={styles.modalSafe}>
          <KeyboardAvoidingView style={styles.panel} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
            <View style={styles.header}>
              <View style={styles.agentMark}>
                <Bot size={20} color="#ffffff" />
              </View>
              <View style={styles.headerText}>
                <AppText weight="bold" numberOfLines={1} style={styles.headerTitle}>
                  {headerTitle}
                </AppText>
              </View>
              <View style={styles.headerActions}>
                <Pressable
                  accessibilityRole="button"
                  accessibilityLabel="Open chat history"
                  disabled={!canOpenHistory}
                  style={({ pressed }) => [styles.iconButton, !canOpenHistory && styles.disabled, pressed && styles.pressed]}
                  onPress={() => setIsHistoryOpen((open) => !open)}
                >
                  <MessagesSquare size={18} color={colors.text} />
                </Pressable>
                <Pressable
                  accessibilityRole="button"
                  accessibilityLabel="Start new chat"
                  disabled={isSending}
                  style={({ pressed }) => [styles.iconButton, isSending && styles.disabled, pressed && styles.pressed]}
                  onPress={handleNewChat}
                >
                  <Plus size={18} color={colors.text} />
                </Pressable>
                <Pressable accessibilityRole="button" accessibilityLabel="Close agent chat" style={styles.iconButton} onPress={closePanel}>
                  <X size={18} color={colors.text} />
                </Pressable>
              </View>
            </View>

            {isHistoryOpen ? (
              <View style={styles.historyPanel}>
                <ScrollView style={styles.historyScroll} showsVerticalScrollIndicator={false}>
                  {chatHistory.map((conversation) => (
                    <View
                      key={conversation.id}
                      style={[
                        styles.historyRow,
                        conversation.id === currentConversationId && styles.optionActive
                      ]}
                    >
                      <Pressable
                        accessibilityRole="button"
                        style={({ pressed }) => [styles.historyOption, pressed && styles.pressed]}
                        onPress={() => handleSelectChat(conversation.id)}
                      >
                        <AppText numberOfLines={2}>{conversation.title}</AppText>
                      </Pressable>
                      <Pressable
                        accessibilityRole="button"
                        accessibilityLabel={`Delete ${conversation.title}`}
                        disabled={isDeletingConversation}
                        style={({ pressed }) => [
                          styles.historyDeleteButton,
                          isDeletingConversation && styles.disabled,
                          pressed && styles.pressed
                        ]}
                        onPress={() => handleRequestDeleteConversation(conversation)}
                      >
                        <Trash2 size={16} color={colors.danger} />
                      </Pressable>
                    </View>
                  ))}
                </ScrollView>
              </View>
            ) : null}

            <FlatList
              ref={messagesListRef}
              data={displayedAgentMessages}
              keyExtractor={(item) => item.id}
              renderItem={renderMessage}
              style={styles.messages}
              contentContainerStyle={[styles.messagesContent, displayedAgentMessages.length === 0 && styles.emptyMessagesContent]}
              onContentSizeChange={() => messagesListRef.current?.scrollToEnd({ animated: true })}
              keyboardShouldPersistTaps="handled"
              ListEmptyComponent={
                <View style={styles.emptyState}>
                  <AppText muted weight="semibold" style={styles.emptyStateText}>
                    ask questions about your budget
                  </AppText>
                </View>
              }
            />

            <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.suggestions}>
              {SUGGESTIONS.map((suggestion) => (
                <Pressable
                  key={suggestion}
                  accessibilityRole="button"
                  disabled={isSending || !hasSelectedBudget}
                  style={({ pressed }) => [
                    styles.suggestionButton,
                    (isSending || !hasSelectedBudget) && styles.disabled,
                    pressed && styles.pressed
                  ]}
                  onPress={() => submitMessage(suggestion)}
                >
                  <AppText muted weight="semibold" style={styles.suggestionText}>
                    {suggestion}
                  </AppText>
                </Pressable>
              ))}
            </ScrollView>

            <View style={styles.composer}>
              <TextInput
                ref={composerInputRef}
                value={composerValue}
                editable={!isSending && hasSelectedBudget}
                placeholder={hasSelectedBudget ? 'How can I help you today?' : 'Select a budget to chat'}
                placeholderTextColor={colors.muted}
                style={styles.composerInput}
                returnKeyType="send"
                blurOnSubmit={false}
                onChangeText={setComposerValue}
                onSubmitEditing={() => submitMessage(composerValue)}
              />

              {isModelOpen ? (
                <View style={styles.modelPanel}>
                  {AGENT_MODEL_OPTIONS.map((modelOption) => (
                    <Pressable
                      key={modelOption.key}
                      accessibilityRole="button"
                      style={[styles.modelOption, modelOption.key === selectedModelKey && styles.optionActive]}
                      onPress={() => handleSelectModel(modelOption.key)}
                    >
                      <AppText numberOfLines={1}>{modelOption.label}</AppText>
                    </Pressable>
                  ))}
                </View>
              ) : null}

              <View style={styles.composerFooter}>
                <Pressable
                  accessibilityRole="button"
                  accessibilityLabel="Select agent model"
                  disabled={!canOpenModels}
                  style={({ pressed }) => [styles.modelButton, !canOpenModels && styles.disabled, pressed && styles.pressed]}
                  onPress={() => setIsModelOpen((open) => !open)}
                >
                  <AppText muted numberOfLines={1} style={styles.modelButtonText}>
                    {selectedModel.shortLabel ?? selectedModel.label}
                  </AppText>
                  <ChevronDown size={14} color={colors.muted} />
                </Pressable>
                <Pressable
                  accessibilityRole="button"
                  accessibilityLabel="Send message"
                  disabled={isSending || !hasSelectedBudget || !composerValue.trim()}
                  style={({ pressed }) => [
                    styles.sendButton,
                    (isSending || !hasSelectedBudget || !composerValue.trim()) && styles.disabled,
                    pressed && styles.pressed
                  ]}
                  onPress={() => submitMessage(composerValue)}
                >
                  <Send size={17} color="#ffffff" />
                </Pressable>
              </View>
            </View>
          </KeyboardAvoidingView>
        </SafeAreaView>
      </Modal>

      <Modal
        transparent
        visible={Boolean(conversationToDelete)}
        animationType="fade"
        onRequestClose={handleCancelDeleteConversation}
      >
        <View style={styles.confirmBackdrop}>
          <View style={styles.confirmDialog}>
            <AppText weight="bold" style={styles.confirmTitle}>
              Delete chat?
            </AppText>
            <AppText muted style={styles.confirmText}>
              This will remove "{deleteConversationTitle}" from your chat history.
            </AppText>
            <View style={styles.confirmActions}>
              <Pressable
                accessibilityRole="button"
                disabled={isDeletingConversation}
                style={({ pressed }) => [styles.confirmCancelButton, isDeletingConversation && styles.disabled, pressed && styles.pressed]}
                onPress={handleCancelDeleteConversation}
              >
                <AppText weight="semibold">Cancel</AppText>
              </Pressable>
              <Pressable
                accessibilityRole="button"
                disabled={isDeletingConversation}
                style={({ pressed }) => [
                  styles.confirmDeleteButton,
                  isDeletingConversation && styles.disabled,
                  pressed && styles.pressed
                ]}
                onPress={handleConfirmDeleteConversation}
              >
                {isDeletingConversation ? (
                  <ActivityIndicator size="small" color="#ffffff" />
                ) : (
                  <AppText weight="bold" style={styles.confirmDeleteText}>
                    Delete
                  </AppText>
                )}
              </Pressable>
            </View>
          </View>
        </View>
      </Modal>
    </>
  );
}

const styles = StyleSheet.create({
  launcher: {
    position: 'absolute',
    right: spacing.lg,
    bottom: 104,
    zIndex: 50,
    elevation: 18,
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 48,
    paddingHorizontal: spacing.lg,
    borderRadius: 999,
    backgroundColor: colors.primary,
    shadowColor: '#000000',
    shadowOffset: { width: 0, height: 10 },
    shadowOpacity: 0.28,
    shadowRadius: 16
  },
  launcherText: {
    color: '#ffffff'
  },
  modalSafe: {
    flex: 1,
    backgroundColor: colors.background
  },
  panel: {
    flex: 1,
    backgroundColor: colors.surface
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.md,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.borderMuted,
    backgroundColor: colors.surface
  },
  agentMark: {
    width: 42,
    height: 42,
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: radii.lg,
    backgroundColor: colors.primary
  },
  headerText: {
    flex: 1,
    minWidth: 0
  },
  headerTitle: {
    fontSize: 17,
    lineHeight: 22
  },
  headerActions: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm
  },
  iconButton: {
    width: 36,
    height: 36,
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: radii.md,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.borderMuted,
    backgroundColor: colors.surfaceStrong
  },
  historyPanel: {
    maxHeight: 184,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.borderMuted,
    backgroundColor: colors.background,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm
  },
  historyScroll: {
    maxHeight: 168
  },
  historyRow: {
    flexDirection: 'row',
    alignItems: 'center',
    minHeight: 44,
    borderRadius: radii.md
  },
  historyOption: {
    flex: 1,
    minHeight: 44,
    justifyContent: 'center',
    paddingLeft: spacing.md,
    paddingRight: spacing.sm,
    paddingVertical: spacing.sm
  },
  historyDeleteButton: {
    width: 38,
    height: 38,
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: radii.md
  },
  optionActive: {
    backgroundColor: colors.surfaceTertiary
  },
  messages: {
    flex: 1,
    backgroundColor: colors.surface
  },
  messagesContent: {
    flexGrow: 1,
    gap: spacing.md,
    padding: spacing.lg
  },
  emptyMessagesContent: {
    justifyContent: 'center'
  },
  emptyState: {
    alignItems: 'center',
    justifyContent: 'center',
    minHeight: 180
  },
  emptyStateText: {
    opacity: 0.55,
    textAlign: 'center'
  },
  messageBubble: {
    maxWidth: '86%',
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.md,
    borderRadius: 18
  },
  agentMessage: {
    alignSelf: 'flex-start',
    borderTopLeftRadius: radii.sm,
    backgroundColor: '#2c2c2a'
  },
  userMessage: {
    alignSelf: 'flex-end',
    borderTopRightRadius: radii.sm,
    backgroundColor: colors.primary
  },
  messageLabel: {
    marginBottom: spacing.xs,
    color: colors.muted,
    fontSize: 12,
    lineHeight: 16,
    textTransform: 'uppercase'
  },
  messageText: {
    lineHeight: 21
  },
  userMessageText: {
    color: '#ffffff',
    lineHeight: 21
  },
  messageParts: {
    gap: spacing.sm
  },
  loadingMessage: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
    minHeight: 24
  },
  loadingText: {
    fontSize: 14,
    lineHeight: 18
  },
  toolPart: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    gap: spacing.sm,
    padding: spacing.sm,
    borderRadius: radii.md,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.borderMuted,
    backgroundColor: colors.background
  },
  toolPartPending: {
    borderColor: colors.primaryLight
  },
  toolText: {
    flex: 1,
    minWidth: 0
  },
  toolName: {
    fontSize: 14,
    lineHeight: 18
  },
  toolSummary: {
    fontSize: 13,
    lineHeight: 18
  },
  suggestions: {
    gap: spacing.sm,
    paddingHorizontal: spacing.lg,
    paddingVertical: spacing.md,
    backgroundColor: colors.surface
  },
  suggestionButton: {
    minHeight: 36,
    justifyContent: 'center',
    paddingHorizontal: spacing.md,
    borderRadius: 999,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.borderMuted
  },
  suggestionText: {
    fontSize: 13,
    lineHeight: 17
  },
  composer: {
    gap: spacing.md,
    margin: spacing.md,
    padding: spacing.lg,
    borderRadius: 22,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.borderMuted,
    backgroundColor: '#2c2c2a'
  },
  composerInput: {
    minHeight: 42,
    padding: 0,
    color: colors.text,
    fontSize: 16,
    lineHeight: 21
  },
  composerFooter: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'flex-end',
    gap: spacing.md
  },
  modelPanel: {
    gap: spacing.xs,
    maxHeight: 184,
    padding: spacing.sm,
    borderRadius: radii.lg,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.borderMuted,
    backgroundColor: colors.background
  },
  modelOption: {
    minHeight: 38,
    justifyContent: 'center',
    paddingHorizontal: spacing.md,
    borderRadius: radii.md
  },
  modelButton: {
    flexDirection: 'row',
    alignItems: 'center',
    flexShrink: 1,
    gap: spacing.xs,
    minHeight: 38,
    maxWidth: '72%',
    paddingHorizontal: spacing.sm,
    borderRadius: 999
  },
  modelButtonText: {
    flexShrink: 1,
    fontSize: 14
  },
  sendButton: {
    width: 40,
    height: 40,
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: 20,
    backgroundColor: colors.primary
  },
  disabled: {
    opacity: 0.5
  },
  pressed: {
    opacity: 0.78
  },
  confirmBackdrop: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    padding: spacing.lg,
    backgroundColor: 'rgba(0, 0, 0, 0.48)'
  },
  confirmDialog: {
    width: '100%',
    maxWidth: 360,
    gap: spacing.md,
    padding: spacing.lg,
    borderRadius: radii.lg,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.borderMuted,
    backgroundColor: colors.surface
  },
  confirmTitle: {
    fontSize: 18,
    lineHeight: 24
  },
  confirmText: {
    lineHeight: 21
  },
  confirmActions: {
    flexDirection: 'row',
    justifyContent: 'flex-end',
    gap: spacing.md
  },
  confirmCancelButton: {
    minHeight: 42,
    justifyContent: 'center',
    paddingHorizontal: spacing.lg,
    borderRadius: radii.md,
    borderWidth: StyleSheet.hairlineWidth,
    borderColor: colors.borderMuted
  },
  confirmDeleteButton: {
    minWidth: 88,
    minHeight: 42,
    alignItems: 'center',
    justifyContent: 'center',
    paddingHorizontal: spacing.lg,
    borderRadius: radii.md,
    backgroundColor: colors.danger
  },
  confirmDeleteText: {
    color: '#ffffff'
  }
});
