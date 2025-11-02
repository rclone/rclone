---
title: "Privacy Policy"
description: "Rclone Privacy Policy"
---

# Rclone Privacy Policy

## What is this Privacy Policy for?

This privacy policy is for this website <https://rclone.org> and governs the
privacy of its users who choose to use it.

The policy sets out the different areas where user privacy is concerned and
outlines the obligations & requirements of the users, the website and website
owners. Furthermore the way this website processes, stores and protects user
data and information will also be detailed within this policy.

## The Website

This website and its owners take a proactive approach to user privacy and
ensure the necessary steps are taken to protect the privacy of its users
throughout their visiting experience. This website complies to all UK national
laws and requirements for user privacy.

## Use of Cookies

This website uses cookies to better the users experience while visiting the
website. Where applicable this website uses a cookie control system allowing
the user on their first visit to the website to allow or disallow the use of
cookies on their computer / device. This complies with recent legislation
requirements for websites to obtain explicit consent from users before leaving
behind or reading files such as cookies on a user's computer / device.

Cookies are small files saved to the user's computers hard drive that track,
save and store information about the user's interactions and usage of the
website. This allows the website, through its server to provide the users with
a tailored experience within this website.

Users are advised that if they wish to deny the use and saving of cookies from
this website on to their computers hard drive they should take necessary steps
within their web browsers security settings to block all cookies from this
website and its external serving vendors.

This website uses tracking software to monitor its visitors to better
understand how they use it. This software is provided by Google Analytics which
uses cookies to track visitor usage. The software will save a cookie to your
computers hard drive in order to track and monitor your engagement and usage of
the website, but will not store, save or collect personal information. You can
read [Google's privacy policy here](https://www.google.com/privacy.html) for
further information.

Other cookies may be stored to your computers hard drive by external vendors
when this website uses referral programs, sponsored links or adverts. Such
cookies are used for conversion and referral tracking and typically expire
after 30 days, though some may take longer. No personal information is stored,
saved or collected.

## Contact & Communication

Users contacting this website and/or its owners do so at their own discretion
and provide any such personal details requested at their own risk. Your
personal information is kept private and stored securely until a time it is no
longer required or has no use, as detailed in the Data Protection Act 1998.

This website and its owners use any information submitted to provide you with
further information about the products / services they offer or to assist you
in answering any questions or queries you may have submitted.

## External Links

Although this website only looks to include quality, safe and relevant external
links, users are advised adopt a policy of caution before clicking any external
web links mentioned throughout this website.

The owners of this website cannot guarantee or verify the contents of any
externally linked website despite their best efforts. Users should therefore
note they click on external links at their own risk and this website and its
owners cannot be held liable for any damages or implications caused by visiting
any external links mentioned.

## Adverts and Sponsored Links

This website may contain sponsored links and adverts. These will typically be
served through our advertising partners, to whom may have detailed privacy
policies relating directly to the adverts they serve.

Clicking on any such adverts will send you to the advertisers website through a
referral program which may use cookies and will track the number of referrals
sent from this website. This may include the use of cookies which may in turn
be saved on your computers hard drive. Users should therefore note they click
on sponsored external links at their own risk and this website and its owners
cannot be held liable for any damages or implications caused by visiting any
external links mentioned.

### Social Media Platforms

Communication, engagement and actions taken through external social media
platforms that this website and its owners participate on are subject to the
terms and conditions as well as the privacy policies held with each social media
platform respectively.

Users are advised to use social media platforms wisely and communicate / engage
upon them with due care and caution in regard to their own privacy and personal
details. This website nor its owners will ever ask for personal or sensitive
information through social media platforms and encourage users wishing to
discuss sensitive details to contact them through primary communication channels
such as email.

This website may use social sharing buttons which help share web content
directly from web pages to the social media platform in question. Users are
advised before using such social sharing buttons that they do so at their own
discretion and note that the social media platform may track and save your
request to share a web page respectively through your social media platform
account.

## Use of Cloud API User Data

Rclone is a command-line program to manage files on cloud storage. Its sole
purpose is to access and manipulate user content in the [supported](/overview/)
cloud storage systems from a local machine of the end user. For accessing the
user content via the cloud provider API, Rclone uses authentication mechanisms,
such as OAuth or HTTP Cookies, depending on the particular cloud provider
offerings. Use of these authentication mechanisms and user data is governed by
the privacy policies mentioned in the [Resources & Further Information](/privacy/#resources-further-information)
section and followed by the privacy policy of Rclone.

- Rclone provides the end user with access to their files available in a storage
  system associated by the authentication credentials via the publicly exposed API
  of the storage system.
- Rclone allows storing the authentication credentials on the user machine in the
  local configuration file.
- Rclone does not share any user data with third parties.

## User Data Collection and Storage

This section outlines how rclone accesses, uses, stores, and shares
user data obtained from service provider APIs. Our use of information
received from provider APIs will adhere to the provider API Services
User Data Policy, including the Limited Use requirements.

Rclone is a client-side command-line program that users run on their
own computers to manage their files on cloud storage services. The
rclone project does not operate any servers that store or process your
personal data. All data access and processing occurs directly on the
user's machine and between the user's machine and the provider API
servers.

### Data Accessed

When you authorize rclone to access your files on your provider, it
may access the following types of data, depending on the permissions
you grant:

- Files: Rclone accesses the metadata (filenames, sizes, modification
  times, etc.) and content of your files and folders on your provider.
  This is necessary for rclone to perform file management tasks like
  copying, syncing, moving, and listing files.

- Authentication Tokens: Rclone requests OAuth 2.0 access tokens from
  the provider. These tokens are used to authenticate your requests to
  the provider's APIs and prove that you have granted rclone
  permission to access your data.

- Basic Profile Information: As part of the authentication process,
  rclone may receive your email address to identify the connected
  account within the rclone configuration.

### Data Usage

Rclone uses the user data it accesses solely to provide its core
functionality, which is initiated and controlled entirely by you, the
user. Specifically:

- The data is used to perform file transfer and management operations
  (such as `copy`, `sync`, `move`, `list`, `delete`) between your
  local machine and your provider account as per your direct commands.

- Authentication tokens are used exclusively to make authorized API
  calls to the provider's services on your behalf.

- Your email address is used locally to help you identify which
  provider account is configured.

Rclone does not use your data for any other purpose, such as
advertising, marketing, or analysis by the rclone project developers.

### Data Sharing

Rclone does not share your user data with any third parties.

All data transfers initiated by the user occur directly between the
machine where rclone is running and the provider's servers. The rclone
project and its developers **never** have access to your
authentication tokens or your file data.

### Data Storage & Protection

- Configuration Data: Rclone stores its configuration, including the
  OAuth 2.0 tokens required to access your provider account, in a
  configuration file (`rclone.conf`) located on your local machine.

- Security: You are responsible for securing this configuration
  file on your own computer. Rclone provides a built-in option to
  encrypt the configuration file with a password for an added layer of
  security. We strongly recommend using this feature.

- File Data: Your file data is only held in your computer's memory
  (RAM) temporarily during transfer operations. Rclone does not
  permanently store your file content on your local disk unless you
  explicitly command it to do so (e.g., by running a `copy` command
  from the provider to a local directory).

### Data Retention & Deletion

Rclone gives you full control over your data.

- Data Retention: Rclone retains the configuration data, including
  authentication tokens, on your local machine for as long as you keep
  the configuration file. This allows you to use rclone without having
  to re-authenticate for every session.

- Data Deletion: You can delete your data and revoke rclone's
  access at any time through one of the following methods:

    1. Local Deletion: You can delete the specific provider
       configuration from your `rclone.conf` file or delete the entire
       file itself. This will permanently remove the authentication
       tokens from your machine.

    2. Revoking Access via the provider: You can revoke rclone's
       access to your provider directly from your the providers's
       security settings page. This will invalidate the authentication
       tokens, and rclone will no longer be able to access your data.
       For example, if you are using Google you can manage your permissions
       [on the Google permissions page](https://myaccount.google.com/permissions).

## Resources & Further Information

- [Data Protection Act 1998](http://www.legislation.gov.uk/ukpga/1998/29/contents)
- [Privacy and Electronic Communications Regulations 2003](http://www.legislation.gov.uk/uksi/2003/2426/contents/made)
- [Privacy and Electronic Communications Regulations 2003 - The Guide](https://ico.org.uk/for-organisations/guide-to-pecr/)
- [Twitter Privacy Policy](https://twitter.com/privacy)
- [Facebook Privacy Policy](https://www.facebook.com/about/privacy/)
- [Google Privacy Policy](https://www.google.com/privacy.html)
- [Google API Services User Data Policy](https://developers.google.com/terms/api-services-user-data-policy)
- [Sample Website Privacy Policy](http://www.jamieking.co.uk/resources/free_sample_privacy_policy.html)
