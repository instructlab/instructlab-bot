// src/app/playground/ragchat/page.tsx
'use client';

import React, { useState, useRef, useEffect } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { Button } from '@patternfly/react-core/dist/dynamic/components/Button';
import { Form, FormGroup } from '@patternfly/react-core/dist/dynamic/components/Form';
import { TextInput, TextArea } from '@patternfly/react-core/';
import { Select } from '@patternfly/react-core/dist/dynamic/components/Select';
import { SelectOption, SelectList } from '@patternfly/react-core/dist/dynamic/components/Select';
import { MenuToggle, MenuToggleElement } from '@patternfly/react-core/dist/dynamic/components/MenuToggle';
import { Spinner } from '@patternfly/react-core/dist/dynamic/components/Spinner';
import UserIcon from '@patternfly/react-icons/dist/dynamic/icons/user-icon';
import CopyIcon from '@patternfly/react-icons/dist/dynamic/icons/copy-icon';
import Image from 'next/image';
import styles from './ragchat.module.css';

interface Message {
  text: string;
  isUser: boolean;
}

interface Model {
  name: string;
  apiURL: string;
  modelName: string;
}

const ChatPage: React.FC = () => {
  const [question, setQuestion] = useState('');
  const [systemRole, setSystemRole] = useState(
    'You are a cautious assistant. You carefully follow instructions. You are helpful and harmless and you follow' +
      ' ethical guidelines and promote positive behavior. Given the following information from relevant documentation,' +
      " answer the user's question using only that information, outputted in markdown format. If you are unsure and" +
      ' the answer is not explicitly written in the documentation, say "Sorry, I don\'t have any documentation on that' +
      ' topic." Always include citations from the documentation after you answer the user\'s query.'
  );
  const [messages, setMessages] = useState<Message[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isSelectOpen, setIsSelectOpen] = useState(false);
  const [selectedModel, setSelectedModel] = useState<Model | null>(null);
  const [customModels, setCustomModels] = useState<Model[]>([]);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const messagesContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const fetchDefaultModels = async () => {
      const response = await fetch('/api/envConfig');
      const envConfig = await response.json();

      const defaultModels: Model[] = [
        { name: 'Granite-7b', apiURL: envConfig.GRANITE_API, modelName: envConfig.GRANITE_MODEL_NAME },
        { name: 'Merlinite-7b', apiURL: envConfig.MERLINITE_API, modelName: envConfig.MERLINITE_MODEL_NAME },
      ];

      const storedEndpoints = localStorage.getItem('endpoints');

      const customModels = storedEndpoints
        ? JSON.parse(storedEndpoints).map((endpoint: any) => ({
            name: endpoint.modelName,
            apiURL: `${endpoint.url}`,
            modelName: endpoint.modelName,
          }))
        : [];

      const allModels = [...defaultModels, ...customModels];
      setCustomModels(allModels);
      setSelectedModel(allModels[0] || null);
    };

    fetchDefaultModels();
  }, []);

  const onToggleClick = () => {
    setIsSelectOpen(!isSelectOpen);
  };

  const onSelect = (_event: React.MouseEvent<Element, MouseEvent> | undefined, value: string | number | undefined) => {
    const selected = customModels.find((model) => model.name === value) || null;
    setSelectedModel(selected);
    setIsSelectOpen(false);
  };

  const toggle = (toggleRef: React.Ref<MenuToggleElement>) => (
    <MenuToggle ref={toggleRef} onClick={onToggleClick} isExpanded={isSelectOpen} style={{ width: '200px' }}>
      {selectedModel ? selectedModel.name : 'Select a model'}
    </MenuToggle>
  );

  const dropdownItems = customModels
    .filter((model) => model.name && model.apiURL && model.modelName)
    .map((model, index) => (
      <SelectOption key={index} value={model.name}>
        {model.name}
      </SelectOption>
    ));

  const handleQuestionChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setQuestion(event.target.value);
  };

  const handleSystemRoleChange = (value: string) => {
    setSystemRole(value);
  };

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (event.target.files) {
      setSelectedFile(event.target.files[0]);
    }
  };

  const handleFileUpload = async () => {
    if (!selectedFile) return;

    const formData = new FormData();
    formData.append('file', selectedFile);

    const response = await fetch('/api/playground/ragchat/upload', {
      method: 'POST',
      body: formData,
    });

    if (response.ok) {
      console.log('File uploaded successfully');
    } else {
      console.error('Failed to upload file');
    }
  };

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!question.trim() || !selectedModel) return;

    setMessages((messages) => [...messages, { text: question, isUser: true }]);
    setQuestion('');

    setIsLoading(true);
    const response = await fetch(
      `/api/playground/ragchat?apiURL=${encodeURIComponent(selectedModel.apiURL)}&modelName=${encodeURIComponent(selectedModel.modelName)}`,
      {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ question, systemRole }),
      }
    );

    if (response.body) {
      const reader = response.body.getReader();
      const textDecoder = new TextDecoder('utf-8');
      let botMessage = '';

      setMessages((messages) => [...messages, { text: '', isUser: false }]);

      (async () => {
        for (;;) {
          const { value, done } = await reader.read();
          if (done) break;
          const chunk = textDecoder.decode(value, { stream: true });
          botMessage += chunk;

          setMessages((messages) => {
            const updatedMessages = [...messages];
            updatedMessages[updatedMessages.length - 1].text = botMessage;
            return updatedMessages;
          });
        }
        setIsLoading(false);
      })();
    } else {
      setMessages((messages) => [...messages, { text: 'Failed to fetch response from the server.', isUser: false }]);
      setIsLoading(false);
    }
  };

  useEffect(() => {
    if (messagesContainerRef.current) {
      messagesContainerRef.current.scrollTop = messagesContainerRef.current.scrollHeight;
    }
  }, [messages]);

  const handleCopyToClipboard = (text: string) => {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard
        .writeText(text)
        .then(() => {
          console.log('Text copied to clipboard');
        })
        .catch((err) => {
          console.error('Could not copy text: ', err);
        });
    } else {
      const textArea = document.createElement('textarea');
      textArea.value = text;
      document.body.appendChild(textArea);
      textArea.focus();
      textArea.select();
      try {
        document.execCommand('copy');
        console.log('Text copied to clipboard');
      } catch (err) {
        console.error('Could not copy text: ', err);
      }
      document.body.removeChild(textArea);
    }
  };

  return (
    <AppLayout>
      <div className={styles.chatContainer}>
        <div className={styles.modelAndUploadContainer}>
          <div className={styles.modelSelector}>
            <span className={styles.modelSelectorLabel}>Model Selector</span>
            <Select
              id="single-select"
              isOpen={isSelectOpen}
              selected={selectedModel ? selectedModel.name : 'Select a model'}
              onSelect={onSelect}
              onOpenChange={(isOpen) => setIsSelectOpen(isOpen)}
              toggle={toggle}
              shouldFocusToggleOnSelect
            >
              <SelectList>{dropdownItems}</SelectList>
            </Select>
          </div>
          <div className={styles.fileUpload}>
            <span className={styles.boldLabel}>Upload PDF</span>
            <input type="file" onChange={handleFileChange} />
            <Button variant="secondary" onClick={handleFileUpload} size="sm">
              Upload
            </Button>
          </div>
        </div>
        <FormGroup fieldId="system-role-field" label={<span className={styles.boldLabel}>System Role</span>}>
          <TextArea
            isRequired
            id="system-role-field"
            name="system-role-field"
            value={systemRole}
            onChange={(event) => handleSystemRoleChange(event.currentTarget.value)}
            placeholder="Enter system role..."
            aria-label="System Role"
            rows={4}
          />
        </FormGroup>
        <div ref={messagesContainerRef} className={styles.messagesContainer}>
          {messages.map((msg, index) => (
            <div key={index} className={`${styles.message} ${msg.isUser ? styles.chatQuestion : styles.chatAnswer}`}>
              {msg.isUser ? (
                <UserIcon className={styles.userIcon} />
              ) : (
                <Image src="/bot-icon-chat-32x32.svg" alt="Bot" width={32} height={32} className={styles.botIcon} />
              )}
              <pre>
                <code>{msg.text}</code>
              </pre>
              {!msg.isUser && (
                <Button variant="plain" onClick={() => handleCopyToClipboard(msg.text)} aria-label="Copy to clipboard">
                  <CopyIcon />
                </Button>
              )}
            </div>
          ))}
          {isLoading && <Spinner aria-label="Loading" size="lg" />}
        </div>
        <Form onSubmit={handleSubmit} className={styles.chatForm}>
          <FormGroup fieldId="question-field">
            <TextInput
              isRequired
              type="text"
              id="question-field"
              name="question-field"
              value={question}
              onChange={handleQuestionChange}
              placeholder="Type your question here..."
            />
          </FormGroup>
          <Button variant="primary" type="submit">
            Send
          </Button>
        </Form>
      </div>
    </AppLayout>
  );
};

export default ChatPage;
