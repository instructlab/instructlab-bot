import { exec } from "node:child_process";

import dotenv from "dotenv";
import { Probot } from "probot";

dotenv.config();

function run(cmd: string): Promise<string> {
  return new Promise((resolve, reject) => {
    exec(cmd, (error, stdout, stderr) => {
      if (error) return reject(error);
      if (stderr) console.log(stderr);
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
          `Beep, boop 🤖  Generating test data for your pull request.\n\n` +
          `This may take a few seconds...`,
      });
      await context.octokit.issues.createComment(issueComment);

      try {
        const prNumber = context.payload.issue.number;
        let optionalArgs = ``;
        if (process.env.WORK_DIR != null) {
          optionalArgs = ` --work-dir ${process.env.WORK_DIR}`;
        }
        if (process.env.VENV_DIR != null) {
          optionalArgs = optionalArgs + ` --venv-dir ${process.env.VENV_DIR}`;
        }
        const stdout: string = await run(`./scripts/generate.sh ${optionalArgs} ${prNumber}`);
        let url = "";
        // Set url to the last line of stdout
        if (stdout != null) {
          url = stdout.trim().split("\n").slice(-1)[0];
        }
        // Make sure url starts with https://
        if (url.startsWith("https://")) {
          const issueComment = context.issue({
            body:
              `Beep, boop 🤖  The test data has been generated!\n\n` +
              `Find your results [here](${url}).`,
          });
          await context.octokit.issues.createComment(issueComment);
        } else {
          const issueComment = context.issue({
            body:
              `Beep, boop 🤖  An error occurred executing your command.\n` +
              `\`\`\`console\n` +
              stdout +
              `\n\`\`\`\n`,
          });
          await context.octokit.issues.createComment(issueComment);
        }
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
