import { Probot } from "probot";

export default (app: Probot) => {
  app.on("pull_request.opened", async (context) => {
    const issueComment = context.issue({
      body: `
        Beep, boop 🤖  Hi, I'm instruct-lab-bot and I'm going to help you
        with your pull request. Thanks for you contribution! 🎉

        In order to proceed please reply with the following comment:

        @instruct-lab-bot generate

        This will trigger the generation of some test data for your
        contribution. Once the data is generated, I will let you know
        and you can proceed with the review.
    `,
    });
    await context.octokit.issues.createComment(issueComment);
  });
  app.on("issue_comment.created", async (context) => {
    const issueComment = context.payload.comment.body;
    if (issueComment === "@instruct-lab-bot generate") {
      const issueComment = context.issue({
        body: `
          Beep, boop 🤖  Generating test data for your pull request. This may take a few seconds...
        `,
      });
      await context.octokit.issues.createComment(issueComment);
    } else if (issueComment.startsWith("@instruct-lab-bot")) {
      const issueComment = context.issue({
        body: `
          Beep, boop 🤖  Sorry, I don't understand that command. Please reply with the following comment:

          @instruct-lab-bot generate

          This will trigger the generation of some test data for your
          contribution. Once the data is generated, I will let you know
          and you can proceed with the review.
        `,
      });
      await context.octokit.issues.createComment(issueComment);
    }
    // Don't process the command if it's not for the bot
  });
};
