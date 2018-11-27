[![Logo](https://rclone.org/img/rclone-120x120.png)](https://rclone.org/)

[Website](https://rclone.org) |
[Documentation](https://rclone.org/docs/) |
[Download](https://rclone.org/downloads/) | 
[Contributing](CONTRIBUTING.md) |
[Changelog](https://rclone.org/changelog/) |
[Installation](https://rclone.org/install/) |
[Forum](https://forum.rclone.org/) | 
[G+](https://google.com/+RcloneOrg)

[![Build Status](https://travis-ci.org/ncw/rclone.svg?branch=master)](https://travis-ci.org/ncw/rclone)
[![Windows Build Status](https://ci.appveyor.com/api/projects/status/github/ncw/rclone?branch=master&passingText=windows%20-%20ok&svg=true)](https://ci.appveyor.com/project/ncw/rclone)
[![CircleCI](https://circleci.com/gh/ncw/rclone/tree/master.svg?style=svg)](https://circleci.com/gh/ncw/rclone/tree/master)
[![GoDoc](https://godoc.org/github.com/ncw/rclone?status.svg)](https://godoc.org/github.com/ncw/rclone) 

# Rclone

Rclone *("rsync for cloud storage")* is a command line program to sync files and directories to and from different cloud storage providers.

## Storage providers

  * Amazon Drive [:page_facing_up:](https://rclone.org/amazonclouddrive/) ([See note](https://rclone.org/amazonclouddrive/#status))
  * Amazon S3 [:page_facing_up:](https://rclone.org/s3/)
  * Backblaze B2 [:page_facing_up:](https://rclone.org/b2/)
  * Box [:page_facing_up:](https://rclone.org/box/)
  * Ceph [:page_facing_up:](https://rclone.org/s3/#ceph)
  * DigitalOcean Spaces [:page_facing_up:](https://rclone.org/s3/#digitalocean-spaces)
  * Dreamhost [:page_facing_up:](https://rclone.org/s3/#dreamhost)
  * Dropbox [:page_facing_up:](https://rclone.org/dropbox/)
  * FTP [:page_facing_up:](https://rclone.org/ftp/)
  * Google Cloud Storage [:page_facing_up:](https://rclone.org/googlecloudstorage/)
  * Google Drive [:page_facing_up:](https://rclone.org/drive/)
  * HTTP [:page_facing_up:](https://rclone.org/http/)
  * Hubic [:page_facing_up:](https://rclone.org/hubic/)
  * Jottacloud [:page_facing_up:](https://rclone.org/jottacloud/)
  * IBM COS S3 [:page_facing_up:](https://rclone.org/s3/#ibm-cos-s3)
  * Memset Memstore [:page_facing_up:](https://rclone.org/swift/)
  * Mega [:page_facing_up:](https://rclone.org/mega/)
  * Microsoft Azure Blob Storage [:page_facing_up:](https://rclone.org/azureblob/)
  * Microsoft OneDrive [:page_facing_up:](https://rclone.org/onedrive/)
  * Minio [:page_facing_up:](https://rclone.org/s3/#minio)
  * Nextcloud [:page_facing_up:](https://rclone.org/webdav/#nextcloud)
  * OVH [:page_facing_up:](https://rclone.org/swift/)
  * OpenDrive [:page_facing_up:](https://rclone.org/opendrive/)
  * Openstack Swift [:page_facing_up:](https://rclone.org/swift/)
  * Oracle Cloud Storage [:page_facing_up:](https://rclone.org/swift/)
  * ownCloud [:page_facing_up:](https://rclone.org/webdav/#owncloud)
  * pCloud [:page_facing_up:](https://rclone.org/pcloud/)
  * put.io [:page_facing_up:](https://rclone.org/webdav/#put-io)
  * QingStor [:page_facing_up:](https://rclone.org/qingstor/)
  * Rackspace Cloud Files [:page_facing_up:](https://rclone.org/swift/)
  * SFTP [:page_facing_up:](https://rclone.org/sftp/)
  * Wasabi [:page_facing_up:](https://rclone.org/s3/#wasabi)
  * WebDAV [:page_facing_up:](https://rclone.org/webdav/)
  * Yandex Disk [:page_facing_up:](https://rclone.org/yandex/)
  * The local filesystem [:page_facing_up:](https://rclone.org/local/)
  * 
Contents
Selected Version
Azure Bot Service
Request payment
12/12/2017
9 minutes to read
Contributors
Duc Cash Vo  Kamran Iqbal  CathyQian  Robert Standefer  Kim Brandl - MSFT all
In this article
Prerequisites
Payment process overview
Payment Bot sample
Requesting payment
User experience
Processing callbacks
Testing a payment bot
Additional resources
 Note
This topic applies to SDK v3 release. You can find the documentation for the latest version of the SDK v4 here.

If your bot enables users to purchase items, it can request payment by including a special type of button within a rich card. This article describes how to send a payment request using the Bot Builder SDK for Node.js.
Prerequisites
Before you can send a payment request using the Bot Builder SDK for Node.js, you must complete these prerequisite tasks.
Register and configure your bot

Update your bot's environment variables for MicrosoftAppId and MicrosoftAppPassword to the app ID and password values that were generated for your bot during the registration process.
 Note
To find your bot's AppID and AppPassword, see MicrosoftAppID and MicrosoftAppPassword.
Create and configure merchant account

Create and activate a Stripe account if you don't have one already.
Sign in to Seller Center with your Microsoft account.
Within Seller Center, connect your account with Stripe.
Within Seller Center, navigate to the Dashboard and copy the value of MerchantID.
Update the PAYMENTS_MERCHANT_ID environment variable to the value that you copied from the Seller Center Dashboard.
Payment process overview
The payment process comprises three distinct parts:
The bot sends a payment request.
The user signs in with a Microsoft account to provide payment, shipping, and contact information. Callbacks are sent to the bot to indicate when the bot needs to perform certain operations (update shipping address, update shipping option, complete payment).
The bot processes the callbacks that it receives, including shipping address update, shipping option update, and payment complete.
Your bot must implement only step one and step three of this process; step two takes place outside the context of your bot.
Payment Bot sample
The Payment Bot sample provides an example of a bot that sends a payment request by using Node.js. To see this sample bot in action, you can try it out in web chat, add it as a Skype contact, or download the payment bot sample and run it locally using the Bot Framework Emulator.
 Note
To complete the end-to-end payment process using the Payment Bot sample in web chat or Skype, you must specify a valid credit card or debit card within your Microsoft account (i.e., a valid card from a U.S. card issuer). Your card will not be charged and the card's CVV will not be verified, because the Payment Bot sample runs in test mode (i.e., PAYMENTS_LIVEMODE is set to false in .env).
The next few sections of this article describe the three parts of the payment process, in the context of the Payment Bot sample.
Requesting payment
Your bot can request payment from a user by sending a message that contains a rich card with a button that specifies type of "payment". This code snippet from the Payment Bot sample creates a message that contains a Hero card with a Buy button that the user can click (or tap) to initiate the payment process.
JavaScript

Copy
var bot = new builder.UniversalBot(connector, (session) => {

  catalog.getPromotedItem().then(product => {

    // Store userId for later, when reading relatedTo to resume dialog with the receipt.
    var cartId = product.id;
    session.conversationData[CartIdKey] = cartId;
    session.conversationData[cartId] = session.message.address.user.id;

    // Create PaymentRequest obj based on product information.
    var paymentRequest = createPaymentRequest(cartId, product);

    var buyCard = new builder.HeroCard(session)
      .title(product.name)
      .subtitle(util.format('%s %s', product.currency, product.price))
      .text(product.description)
      .images([
        new builder.CardImage(session).url(product.imageUrl)
      ])
      .buttons([
        new builder.CardAction(session)
          .title('Buy')
          .type(payments.PaymentActionType)
          .value(paymentRequest)
      ]);

    session.send(new builder.Message(session)
      .addAttachment(buyCard));
  });
});
In this example, the button's type is specified as payments.PaymentActionType, which the app defines as "payment". The button's value is populated by the createPaymentRequest function, which returns a PaymentRequest object that contains information about supported payment methods, details, and options. For more information about implementation details, see app.js within the Payment Bot sample.
This screenshot shows the Hero card (with Buy button) that's generated by the code snippet above.
Payments sample bot
 Important
Any user that has access to the Buy button may use it to initiate the payment process. Within the context of a group conversation, it is not possible to designate a button for use by only a specific user.
User experience
When a user clicks the Buy button, he or she is directed to the payment web experience to provide all required payment, shipping, and contact information via their Microsoft account.
Microsoft payment
HTTP callbacks

HTTP callbacks will be sent to your bot to indicate that it should perform certain operations. Each callback will be an event that contains these property values:
Property	Value
type	invoke
name	Indicates the type of operation that the bot should perform (e.g., shipping address update, shipping option update, payment complete).
value	The request payload in JSON format.
relatesTo	Describes the channel and user that are associated with the payment request.
 Note
invoke is a special event type that is reserved for use by the Microsoft Bot Framework. The sender of an invoke event will expect your bot to acknowledge the callback by sending an HTTP response.
Processing callbacks
When your bot receives a callback, it should verify that the information specified in the callback is valid and acknowledge the callback by sending an HTTP response.
Shipping Address Update and Shipping Option Update callbacks

When receiving a Shipping Address Update or a Shipping Option Update callback, your bot will be provided with the current state of the payment details from the client in the event's value property. As a merchant, you should treat these callbacks as static, given input payment details you will calculate some output payment details and fail if the input state provided by the client is invalid for any reason.  If the bot determines the given information is valid as-is, simply send HTTP status code 200 OK along with the unmodified payment details. Alternatively, the bot may send HTTP status code 200 OK along with an updated payment details that should be applied before the order can be processed. In some cases, your bot may determine that the updated information is invalid and the order cannot be processed as-is. For example, the user's shipping address may specify a country to which the product supplier does not ship. In that case, the bot may send HTTP status code 200 OK and a message populating the error property of the payment details object. Sending any HTTP status code in the 400 or 500 range to will result in a generic error for the customer.
Payment Complete callbacks

When receiving a Payment Complete callback, your bot will be provided with a copy of the initial, unmodified payment request as well as the payment response objects in the event's value property. The payment response object will contain the final selections made by the customer along with a payment token. Your bot should take the opportunity to recalculate the final payment request based on the initial payment request and the customer's final selections. Assuming the customer's selections are determined to be valid, the bot should verify the amount and currency in the payment token header to ensure that they match the final payment request. If the bot decides to charge the customer it should only charge the amount in the payment token header as this is the price the customer confirmed. If there is a mismatch between the values that the bot expects and the values that it received in the Payment Complete callback, it can fail the payment request by sending HTTP status code 200 OK along with setting the result field to failure.
In addition to verifying payment details, the bot should also verify that the order can be fulfilled, before it initiates payment processing. For example, it may want to verify that the item(s) being purchased are still available in stock. If the values are correct and your payment processor has successfully charged the payment token, then the bot should respond with HTTP status code 200 OK along with setting the result field to success in order for the payment web experience to display the payment confirmation. The payment token that the bot receives can only be used once, by the merchant that requested it, and must be submitted to Stripe (the only payment processor that the Bot Framework currently supports). Sending any HTTP status code in the 400 or 500 range to will result in a generic error for the customer.
This code snippet from the Payment Bot sample processes the callbacks that the bot receives.
JavaScript

Copy
connector.onInvoke((invoke, callback) => {
  console.log('onInvoke', invoke);

  // This is a temporary workaround for the issue that the channelId for "webchat" is mapped to "directline" in the incoming RelatesTo object
  invoke.relatesTo.channelId = invoke.relatesTo.channelId === 'directline' ? 'webchat' : invoke.relatesTo.channelId;

  var storageCtx = {
    address: invoke.relatesTo,
    persistConversationData: true,
    conversationId: invoke.relatesTo.conversation.id
  };

  connector.getData(storageCtx, (err, data) => {
    var cartId = data.conversationData[CartIdKey];
    if (!invoke.relatesTo.user && cartId) {
      // Bot keeps the userId in context.ConversationData[cartId]
      var userId = data.conversationData[cartId];
      invoke.relatesTo.useAuth = true;
      invoke.relatesTo.user = { id: userId };
    }

    // Continue based on PaymentRequest event.
    var paymentRequest = null;
    switch (invoke.name) {
      case payments.Operations.UpdateShippingAddressOperation:
      case payments.Operations.UpdateShippingOptionOperation:
        paymentRequest = invoke.value;

        // Validate address AND shipping method (if selected).
        checkout
          .validateAndCalculateDetails(paymentRequest, paymentRequest.shippingAddress, paymentRequest.shippingOption)
          .then(updatedPaymentRequest => {
            // Return new paymentRequest with updated details.
            callback(null, updatedPaymentRequest, 200);
          }).catch(err => {
            // Return error to onInvoke handler.
            callback(err);
            // Send error message back to user.
            bot.beginDialog(invoke.relatesTo, 'checkout_failed', {
              errorMessage: err.message
            });
          });

        break;

      case payments.Operations.PaymentCompleteOperation:
        var paymentRequestComplete = invoke.value;
        paymentRequest = paymentRequestComplete.paymentRequest;
        var paymentResponse = paymentRequestComplete.paymentResponse;

        // Validate address AND shipping method.
        checkout
          .validateAndCalculateDetails(paymentRequest, paymentResponse.shippingAddress, paymentResponse.shippingOption)
          .then(updatedPaymentRequest =>
            // Process payment.
            checkout
              .processPayment(updatedPaymentRequest, paymentResponse)
              .then(chargeResult => {
                // Return success.
                callback(null, { result: "success" }, 200);
                // Send receipt to user.
                bot.beginDialog(invoke.relatesTo, 'checkout_receipt', {
                  paymentRequest: updatedPaymentRequest,
                  chargeResult: chargeResult
                });
              })
          ).catch(err => {
            // Return error to onInvoke handler.
            callback(err);
            // Send error message back to user.
            bot.beginDialog(invoke.relatesTo, 'checkout_failed', {
              errorMessage: err.message
            });
          });

        break;
    }

  });
});
In this example, the bot examines the name property of the incoming event to identify the type of operation it needs to perform, and then calls the appropriate method(s) to process the callback. For more information about implementation details, see app.js within the Payment Bot sample.
Testing a payment bot
To fully test a bot that requests payment, configure it to run on channels that support Bot Framework payments, like Web Chat and Skype. Alternatively, you can test your bot locally using the Bot Framework Emulator.
 Tip
Callbacks are sent to your bot when a user changes data or clicks Pay during the payment web experience. Therefore, you can test your bot's ability to receive and process callbacks by interacting with the payment web experience yourself.
In the Payment Bot sample, the PAYMENTS_LIVEMODE environment variable in .env determines whether Payment Complete callbacks will contain emulated payment tokens or real payment tokens. If PAYMENTS_LIVEMODE is set to false, a header is added to the bot's outbound payment request to indicate that the bot is in test mode, and the Payment Complete callback will contain an emulated payment token that cannot be charged. If PAYMENTS_LIVEMODE is set to true, the header which indicates that the bot is in test mode is omitted from the bot's outbound payment request, and the Payment Complete callback will contain a real payment token that the bot will submit to Stripe for payment processing. This will be a real transaction that results in charges to the specified payment instrument.
Additional resources
Payment Bot sample
Add rich card attachments to messages
Web Payments at W3C
Feedback

Would you like to provide feedback?

Sign in to give feedback
 
Our new feedback system is built on GitHub Issues. Read about this change in our blog post.
There is currently no feedback for this document. Submitted feedback will appear here.
Feedback
English (United States)
Previous Version Docs Blog Contribute Privacy & Cookies Terms of Use Site Feedback Trademarks
## Features

  * MD5/SHA1 hashes checked at all times for file integrity
  * Timestamps preserved on files
  * Partial syncs supported on a whole file basis
  * [Copy](https://rclone.org/commands/rclone_copy/) mode to just copy new/changed files
  * [Sync](https://rclone.org/commands/rclone_sync/) (one way) mode to make a directory identical
  * [Check](https://rclone.org/commands/rclone_check/) mode to check for file hash equality
  * Can sync to and from network, eg two different cloud accounts
  * Optional encryption ([Crypt](https://rclone.org/crypt/))
  * Optional cache ([Cache](https://rclone.org/cache/))
  * Optional FUSE mount ([rclone mount](https://rclone.org/commands/rclone_mount/))

## Installation & documentation

Please see the rclone website for installation, usage, documentation, 
changelog and configuration walkthroughs.

  * https://rclone.org/

## Downloads

  * https://rclone.org/downloads/

License
-------

This is free software under the terms of MIT the license (check the
[COPYING file](/rclone/COPYING) included in this package).
