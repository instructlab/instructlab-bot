import { exec } from "node:child_process";

import dotenv from "dotenv";
import { Probot } from "probot";

dotenv.config();

const WORK_DIR = process.env.WORK_DIR;
const VENV_DIR = process.env.VENV_DIR;

function run(cmd: string) {
  return new Promise((resolve, reject) => {
    exec(cmd, (error, stdout, stderr) => {
      if (error) return reject(error);
      if (stderr) return reject(stderr);
      resolve(stdout);
    });
  });
}

export default (app: Probot) => {
  app.on("pull_request.opened", async (context) => {
    const issueComment = context.issue({
      body:
        `Beep, boop 🤖  Hi, I'm instruct-lab-bot and I'm going to help you` +
        ` with your pull request. Thanks for you contribution! 🎉\n` +
        `In order to proceed please reply with the following comment:\n` +
        `\`@instruct-lab-bot generate\`\n` +
        `This will trigger the generation of some test data for your` +
        ` contribution. Once the data is generated, I will let you know` +
        ` and you can proceed with the review.`,
    });
    await context.octokit.issues.createComment(issueComment);
  });
  app.on("issue_comment.created", async (context) => {
    const issueComment = context.payload.comment.body;
    if (issueComment === "@instruct-lab-bot generate") {
      if (context.payload.issue.pull_request == null) {
        const issueComment = context.issue({
          body: `Beep, boop 🤖  Sorry, I can only generate test data for pull requests.`,
        });
        await context.octokit.issues.createComment(issueComment);
        return;
      }
      const issueComment = context.issue({
        body:
          `Beep, boop 🤖  Generating test data for your pull request.\n` +
          `This may take a few seconds...`,
      });
      await context.octokit.issues.createComment(issueComment);

      try {
        const prNumber = context.payload.issue.number;
        await run(
          `./scripts/generate.sh --work-dir ${WORK_DIR} --venv-dir ${VENV_DIR} ${prNumber}`,
        );
        const issueComment = context.issue({
          body:
            `Beep, boop 🤖  The test data has been generated!\n` +
            `You can now proceed with the review.`,
        });
        await context.octokit.issues.createComment(issueComment);
      } catch (err) {
        const issueComment = context.issue({
          body:
            `Beep, boop 🤖  An error occurred executing your command.\n` +
            `\`\`\`console\n` +
            err +
            `\n\`\`\`\n`,
        });
        await context.octokit.issues.createComment(issueComment);
        return;
      }
    } else if (issueComment.startsWith("@instruct-lab-bot")) {
      const issueComment = context.issue({
        body: `Beep, boop 🤖  Sorry, I don't understand that command`,
      });
      await context.octokit.issues.createComment(issueComment);
    }
    // Don't process the command if it's not for the bot
  });
};
