// src/components/Contribute/Knowledge/index.tsx
'use client';
import React, { useState } from 'react';
import './knowledge.css';
import { usePostKnowledgePR } from '../../../common/HooksPostKnowledgePR';
import { Alert } from '@patternfly/react-core/dist/dynamic/components/Alert';
import { AlertActionCloseButton } from '@patternfly/react-core/dist/dynamic/components/Alert';
import { ActionGroup, FormFieldGroupExpandable, FormFieldGroupHeader } from '@patternfly/react-core/dist/dynamic/components/Form';
import { Button } from '@patternfly/react-core/dist/dynamic/components/Button';
import { Text } from '@patternfly/react-core/dist/dynamic/components/Text';
import { TextInput } from '@patternfly/react-core/dist/dynamic/components/TextInput';
import { Form } from '@patternfly/react-core/dist/dynamic/components/Form';
import { FormGroup } from '@patternfly/react-core/dist/dynamic/components/Form';
import { TextArea } from '@patternfly/react-core/dist/dynamic/components/TextArea';
import { PlusIcon, MinusCircleIcon } from '@patternfly/react-icons/dist/dynamic/icons/';
import yaml from 'js-yaml';
import { validateFields, validateEmail, validateUniqueItems } from '../../../utils/validation';

export const KnowledgeForm: React.FunctionComponent = () => {
  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [task_description, setTaskDescription] = useState('');
  const [task_details, setTaskDetails] = useState('');
  const [domain, setDomain] = useState('');

  const [repo, setRepo] = useState('');
  const [commit, setCommit] = useState('');
  const [patterns, setPatterns] = useState('');

  const [title_work, setTitleWork] = useState('');
  const [link_work, setLinkWork] = useState('');
  const [revision, setRevision] = useState('');
  const [license_work, setLicenseWork] = useState('');
  const [creators, setCreators] = useState('');

  const [questions, setQuestions] = useState<string[]>(new Array(5).fill(''));
  const [answers, setAnswers] = useState<string[]>(new Array(5).fill(''));
  const [isSuccessAlertVisible, setIsSuccessAlertVisible] = useState(false);
  const [isFailureAlertVisible, setIsFailureAlertVisible] = useState(false);

  const [failure_alert_title, setFailureAlertTitle] = useState('');
  const [failure_alert_message, setFailureAlertMessage] = useState('');

  const [success_alert_title, setSuccessAlertTitle] = useState('');
  const [success_alert_message, setSuccessAlertMessage] = useState('');

  const { postKnowledgePR } = usePostKnowledgePR();

  const handleInputChange = (index: number, type: string, value: string) => {
    switch (type) {
      case 'question':
        setQuestions((prevQuestions) => {
          const updatedQuestions = [...prevQuestions];
          updatedQuestions[index] = value;
          return updatedQuestions;
        });
        break;
      case 'answer':
        setAnswers((prevAnswers) => {
          const updatedAnswers = [...prevAnswers];
          updatedAnswers[index] = value;
          return updatedAnswers;
        });
        break;
      default:
        break;
    }
  };

  const addQuestionAnswerPair = () => {
    setQuestions([...questions, '']);
    setAnswers([...answers, '']);
  };

  const deleteQuestionAnswerPair = (index: number) => {
    setQuestions(questions.filter((_, i) => i !== index));
    setAnswers(answers.filter((_, i) => i !== index));
  };

  const resetForm = () => {
    setEmail('');
    setName('');

    setTaskDescription('');
    setTaskDetails('');
    setDomain('');
    setQuestions(new Array(5).fill(''));
    setAnswers(new Array(5).fill(''));

    setRepo('');
    setCommit('');
    setPatterns('');

    setTitleWork('');
    setLinkWork('');
    setLicenseWork('');
    setCreators('');
    setRevision('');
  };

  const onCloseSuccessAlert = () => {
    setSuccessAlertTitle('Knowledge contribution submitted successfully!');
    setSuccessAlertMessage('Thank you for your contribution!!');
    setIsSuccessAlertVisible(false);
  };

  const onCloseFailureAlert = () => {
    setFailureAlertTitle('Failed to submit your Knowledge contribution!');
    setFailureAlertMessage('Please try again later.');
    setIsFailureAlertVisible(false);
  };

  const handleSubmit = async (event: React.FormEvent<HTMLButtonElement>) => {
    event.preventDefault();

    const infoFields = { email, name, task_description, task_details, domain, repo, commit, patterns };
    const attributionFields = { title_work, link_work, revision, license_work, creators };

    let validation = validateFields(infoFields);
    if (!validation.valid) {
      setFailureAlertTitle('Something went wrong!');
      setFailureAlertMessage(validation.message);
      setIsFailureAlertVisible(true);
      return;
    }

    validation = validateFields(attributionFields);
    if (!validation.valid) {
      setFailureAlertTitle('Something went wrong!');
      setFailureAlertMessage(validation.message);
      setIsFailureAlertVisible(true);
      return;
    }

    validation = validateEmail(email);
    if (!validation.valid) {
      setFailureAlertTitle('Something went wrong!');
      setFailureAlertMessage(validation.message);
      setIsFailureAlertVisible(true);
      return;
    }

    validation = validateUniqueItems(questions, 'questions');
    if (!validation.valid) {
      setFailureAlertTitle('Something went wrong!');
      setFailureAlertMessage(validation.message);
      setIsFailureAlertVisible(true);
      return;
    }

    validation = validateUniqueItems(answers, 'answers');
    if (!validation.valid) {
      setFailureAlertTitle('Something went wrong!');
      setFailureAlertMessage(validation.message);
      setIsFailureAlertVisible(true);
      return;
    }

    const [res, err] = await postKnowledgePR({
      name: name,
      email: email,
      task_description: task_description,
      task_details: task_details,
      repo: repo,
      commit: commit,
      patterns: patterns,
      title_work: title_work,
      link_work: link_work,
      revision: revision,
      license_work: license_work,
      creators: creators,
      domain: domain,
      questions,
      answers,
    });

    if (err !== null) {
      setFailureAlertTitle('Failed to submit your Knowledge contribution!');
      setFailureAlertMessage(err);
      setIsFailureAlertVisible(true);
      return;
    }

    if (res !== null) {
      setSuccessAlertTitle('Knowledge contribution submitted successfully!');
      setSuccessAlertMessage(res);
      setIsSuccessAlertVisible(true);
      resetForm();
    }
    console.log('Knowledge submitted successfully : ' + res);
  };

  const handleDownloadYaml = () => {
    const infoFields = { email, name, task_description, task_details, domain, repo, commit, patterns };
    const attributionFields = { title_work, link_work, revision, license_work, creators };

    let validation = validateFields(infoFields);
    if (!validation.valid) {
      setFailureAlertTitle('Something went wrong!');
      setFailureAlertMessage(validation.message);
      setIsFailureAlertVisible(true);
      return;
    }

    validation = validateFields(attributionFields);
    if (!validation.valid) {
      setFailureAlertTitle('Something went wrong!');
      setFailureAlertMessage(validation.message);
      setIsFailureAlertVisible(true);
      return;
    }

    validation = validateEmail(email);
    if (!validation.valid) {
      setFailureAlertTitle('Something went wrong!');
      setFailureAlertMessage(validation.message);
      setIsFailureAlertVisible(true);
      return;
    }

    validation = validateUniqueItems(questions, 'questions');
    if (!validation.valid) {
      setFailureAlertTitle('Something went wrong!');
      setFailureAlertMessage(validation.message);
      setIsFailureAlertVisible(true);
      return;
    }

    validation = validateUniqueItems(answers, 'answers');
    if (!validation.valid) {
      setFailureAlertTitle('Something went wrong!');
      setFailureAlertMessage(validation.message);
      setIsFailureAlertVisible(true);
      return;
    }

    interface SeedExample {
      question: string;
      answer: string;
    }

    interface Document {
      repo: string;
      commit: string;
      patterns: string[];
    }

    const yamlData = {
      created_by: email,
      domain: domain,
      task_description: task_description,
      seed_examples: questions.map(
        (question, index): SeedExample => ({
          question,
          answer: answers[index],
        })
      ),
      document: {
        repo: repo,
        commit: commit,
        patterns: patterns.split(',').map((pattern) => pattern.trim()),
      } as Document,
    };

    const yamlString = yaml.dump(yamlData);
    const blob = new Blob([yamlString], { type: 'application/x-yaml' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'knowledge.yaml';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  return (
    <Form className="form-k">
      <FormFieldGroupExpandable
        isExpanded
        toggleAriaLabel="Details"
        header={
          <FormFieldGroupHeader
            titleText={{ text: 'Author Info', id: 'author-info-id' }}
            titleDescription="Provide your information. Needed for GitHub DCO sign-off."
          />
        }
      >
        <FormGroup isRequired key={'author-info-details-id'}>
          <TextInput
            isRequired
            type="email"
            aria-label="email"
            placeholder="Enter your email address"
            value={email}
            onChange={(_event, value) => setEmail(value)}
          />
          <TextInput
            isRequired
            type="text"
            aria-label="name"
            placeholder="Enter your full name"
            value={name}
            onChange={(_event, value) => setName(value)}
          />
        </FormGroup>
      </FormFieldGroupExpandable>
      <FormFieldGroupExpandable
        isExpanded
        toggleAriaLabel="Details"
        header={
          <FormFieldGroupHeader
            titleText={{ text: 'Knowledge Info', id: 'knowledge-info-id' }}
            titleDescription="Provide brief information about the knowledge."
          />
        }
      >
        <FormGroup key={'knowledge-info-details-id'}>
          <TextInput
            isRequired
            type="text"
            aria-label="task_description"
            placeholder="Enter brief description of the knowledge"
            value={task_description}
            onChange={(_event, value) => setTaskDescription(value)}
          />
          <TextInput
            isRequired
            type="text"
            aria-label="domain"
            placeholder="Enter domain information"
            value={domain}
            onChange={(_event, value) => setDomain(value)}
          />
          <TextArea
            isRequired
            type="text"
            aria-label="task_details"
            placeholder="Provide details about the knowledge"
            value={task_details}
            onChange={(_event, value) => setTaskDetails(value)}
          />
        </FormGroup>
      </FormFieldGroupExpandable>

      <FormFieldGroupExpandable
        toggleAriaLabel="Details"
        header={
          <FormFieldGroupHeader
            titleText={{ text: 'Knowledge', id: 'contrib-knowledge-id' }}
            titleDescription="Contribute new knowledge to the taxonomy repository."
          />
        }
      >
        {questions.map((question, index) => (
          <FormGroup key={index}>
            <Text className="heading-k"> Question and Answer: {index + 1}</Text>
            <TextArea
              isRequired
              type="text"
              aria-label={`Question ${index + 1}`}
              placeholder="Enter the question"
              value={questions[index]}
              onChange={(_event, value) => handleInputChange(index, 'question', value)}
            />
            <TextArea
              isRequired
              type="text"
              aria-label={`Answer ${index + 1}`}
              placeholder="Enter the answer"
              value={answers[index]}
              onChange={(_event, value) => handleInputChange(index, 'answer', value)}
            />
            <Button variant="danger" onClick={() => deleteQuestionAnswerPair(index)}>
              <MinusCircleIcon /> Delete
            </Button>
          </FormGroup>
        ))}
        <Button variant="primary" onClick={addQuestionAnswerPair}>
          <PlusIcon /> Add Question and Answer
        </Button>
      </FormFieldGroupExpandable>

      <FormFieldGroupExpandable
        toggleAriaLabel="Details"
        header={
          <FormFieldGroupHeader titleText={{ text: 'Document Info', id: 'doc-info-id' }} titleDescription="Add the relevant document's information" />
        }
      >
        <FormGroup key={'doc-info-details-id'}>
          <TextInput
            isRequired
            type="url"
            aria-label="repo"
            placeholder="Enter repo url where document exists"
            value={repo}
            onChange={(_event, value) => setRepo(value)}
          />
          <TextInput
            isRequired
            type="text"
            aria-label="commit"
            placeholder="Enter the commit sha of the document in that repo"
            value={commit}
            onChange={(_event, value) => setCommit(value)}
          />
          <TextInput
            isRequired
            type="text"
            aria-label="patterns"
            placeholder="Enter the documents name (comma separated)"
            value={patterns}
            onChange={(_event, value) => setPatterns(value)}
          />
        </FormGroup>
      </FormFieldGroupExpandable>
      <FormFieldGroupExpandable
        toggleAriaLabel="Details"
        header={
          <FormFieldGroupHeader
            titleText={{ text: 'Attribution Info', id: 'attribution-info-id' }}
            titleDescription="Provide attribution information."
          />
        }
      >
        <FormGroup isRequired key={'attribution-info-details-id'}>
          <TextInput
            isRequired
            type="text"
            aria-label="title_work"
            placeholder="Enter title of work"
            value={title_work}
            onChange={(_event, value) => setTitleWork(value)}
          />
          <TextInput
            isRequired
            type="url"
            aria-label="link_work"
            placeholder="Enter link to work"
            value={link_work}
            onChange={(_event, value) => setLinkWork(value)}
          />
          <TextInput
            isRequired
            type="text"
            aria-label="revision"
            placeholder="Enter document revision information"
            value={revision}
            onChange={(_event, value) => setRevision(value)}
          />
          <TextInput
            isRequired
            type="text"
            aria-label="license_work"
            placeholder="Enter license of the work"
            value={license_work}
            onChange={(_event, value) => setLicenseWork(value)}
          />
          <TextInput
            isRequired
            type="text"
            aria-label="creators"
            placeholder="Enter creators Name"
            value={creators}
            onChange={(_event, value) => setCreators(value)}
          />
        </FormGroup>
      </FormFieldGroupExpandable>
      {isSuccessAlertVisible && (
        <Alert variant="success" title={success_alert_title} actionClose={<AlertActionCloseButton onClose={onCloseSuccessAlert} />}>
          {success_alert_message}
        </Alert>
      )}
      {isFailureAlertVisible && (
        <Alert variant="danger" title={failure_alert_title} actionClose={<AlertActionCloseButton onClose={onCloseFailureAlert} />}>
          {failure_alert_message}
        </Alert>
      )}
      <ActionGroup>
        <Button variant="primary" type="submit" className="submit-k" onClick={handleSubmit}>
          Submit
        </Button>
        <Button variant="primary" type="button" className="download-k" onClick={handleDownloadYaml}>
          Download YAML
        </Button>
      </ActionGroup>
    </Form>
  );
};
